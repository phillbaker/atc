package containerserver

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	HandshakeTimeout: 5 * time.Second,
}

func (s *Server) HijackContainer(team db.Team) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handle := r.FormValue(":id")

		hLog := s.logger.Session("hijack", lager.Data{
			"handle": handle,
		})

		container, found, err := s.workerClient.FindContainerByHandle(hLog, team.ID(), handle)
		if err != nil {
			hLog.Error("failed-to-find-container", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			hLog.Info("container-not-found")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		hLog.Debug("found-container")

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			hLog.Error("unable-to-upgrade-connection-for-websockets", err)
			return
		}

		defer conn.Close()

		var processSpec atc.HijackProcessSpec
		err = conn.ReadJSON(&processSpec)
		if err != nil {
			hLog.Error("malformed-process-spec", err)
			closeWithErr(hLog, conn, websocket.CloseUnsupportedData, fmt.Sprintf("malformed process spec"))
			return
		}

		hijackRequest := hijackRequest{
			Container: container,
			Process:   processSpec,
		}

		s.hijack(hLog, conn, hijackRequest)
	})
}

type hijackRequest struct {
	Container worker.Container
	Process   atc.HijackProcessSpec
}

func closeWithErr(log lager.Logger, conn *websocket.Conn, code int, reason string) {
	err := conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(code, reason),
		time.Time{},
	)

	if err != nil {
		log.Error("failed-to-close-websocket-connection", err)
	}
}

func (s *Server) hijack(hLog lager.Logger, conn *websocket.Conn, request hijackRequest) {
	hLog = hLog.Session("hijack", lager.Data{
		"handle":  request.Container.Handle(),
		"process": request.Process,
	})

	stdinR, stdinW := io.Pipe()
	defer stdinW.Close()

	inputs := make(chan atc.HijackInput)
	outputs := make(chan atc.HijackOutput)
	exited := make(chan int, 1)
	errs := make(chan error, 1)

	cleanup := make(chan struct{})
	defer close(cleanup)

	outW := &stdoutWriter{
		outputs: outputs,
		done:    cleanup,
	}

	errW := &stderrWriter{
		outputs: outputs,
		done:    cleanup,
	}

	var tty *garden.TTYSpec

	if request.Process.TTY != nil {
		tty = &garden.TTYSpec{
			WindowSize: &garden.WindowSize{
				Columns: request.Process.TTY.WindowSize.Columns,
				Rows:    request.Process.TTY.WindowSize.Rows,
			},
		}
	}

	process, err := request.Container.Run(garden.ProcessSpec{
		Path: request.Process.Path,
		Args: request.Process.Args,
		Env:  request.Process.Env,
		Dir:  request.Process.Dir,

		User: request.Process.User,

		TTY: tty,
	}, garden.ProcessIO{
		Stdin:  stdinR,
		Stdout: outW,
		Stderr: errW,
	})
	if err != nil {
		hLog.Error("failed-to-hijack", err)
		return
	}

	err = request.Container.MarkAsHijacked()
	if err != nil {
		hLog.Error("failed-to-mark-container-as-hijacked", err)
		return
	}

	hLog.Info("hijacked")

	go func() {
		for {
			var input atc.HijackInput
			err := conn.ReadJSON(&input)
			if err != nil {
				break
			}

			select {
			case inputs <- input:
			case <-cleanup:
				return
			}
		}
	}()

	go func() {
		status, err := process.Wait()
		if err != nil {
			errs <- err
		} else {
			exited <- status
		}
	}()

	for {
		select {
		case input := <-inputs:
			if input.Closed {
				stdinW.Close()
			} else if input.TTYSpec != nil {
				err := process.SetTTY(garden.TTYSpec{
					WindowSize: &garden.WindowSize{
						Columns: input.TTYSpec.WindowSize.Columns,
						Rows:    input.TTYSpec.WindowSize.Rows,
					},
				})
				if err != nil {
					conn.WriteJSON(atc.HijackOutput{
						Error: err.Error(),
					})
				}
			} else {
				stdinW.Write(input.Stdin)
			}

		case output := <-outputs:
			err := conn.WriteJSON(output)
			if err != nil {
				return
			}

		case status := <-exited:
			conn.WriteJSON(atc.HijackOutput{
				ExitStatus: &status,
			})

			return

		case err := <-errs:
			conn.WriteJSON(atc.HijackOutput{
				Error: err.Error(),
			})

			return
		}
	}
}

type stdoutWriter struct {
	outputs chan<- atc.HijackOutput
	done    chan struct{}
}

func (writer *stdoutWriter) Write(b []byte) (int, error) {
	chunk := make([]byte, len(b))
	copy(chunk, b)

	output := atc.HijackOutput{
		Stdout: chunk,
	}

	select {
	case writer.outputs <- output:
	case <-writer.done:
	}

	return len(b), nil
}

func (writer *stdoutWriter) Close() error {
	close(writer.done)
	return nil
}

type stderrWriter struct {
	outputs chan<- atc.HijackOutput
	done    chan struct{}
}

func (writer *stderrWriter) Write(b []byte) (int, error) {
	chunk := make([]byte, len(b))
	copy(chunk, b)

	output := atc.HijackOutput{
		Stderr: chunk,
	}

	select {
	case writer.outputs <- output:
	case <-writer.done:
	}

	return len(b), nil
}

func (writer *stderrWriter) Close() error {
	close(writer.done)
	return nil
}
