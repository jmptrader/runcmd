package runcmd

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"code.google.com/p/go.crypto/ssh"
)

type Runner interface {
	Command(cmd string) (CmdWorker, error)
}

type CmdWorker interface {
	Run() ([]string, error)
	Start() error
	Wait() error
	StdinPipe() io.WriteCloser
	StdoutPipe() io.Reader
	StderrPipe() io.Reader
}

type LocalCmd struct {
	stdinPipe  io.WriteCloser
	stdoutPipe io.Reader
	stderrPipe io.Reader
	cmd        *exec.Cmd
}

type RemoteCmd struct {
	stdinPipe  io.WriteCloser
	stdoutPipe io.Reader
	stderrPipe io.Reader
	cmd        string
	session    *ssh.Session
}

type Local struct {
}

type Remote struct {
	serverConn *ssh.Client
}

func (this *Local) Command(cmd string) (CmdWorker, error) {
	if cmd == "" {
		return nil, errors.New("command cannot be empty")
	}
	c := exec.Command(strings.Fields(cmd)[0], strings.Fields(cmd)[1:]...)
	stdinPipe, err := c.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdoutPipe, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderrPipe, err := c.StderrPipe()
	if err != nil {
		return nil, err
	}
	return &LocalCmd{
		stdinPipe,
		stdoutPipe,
		stderrPipe,
		c,
	}, nil
}

func (this *Remote) Command(cmd string) (CmdWorker, error) {
	if cmd == "" {
		return nil, errors.New("command cannot be empty")
	}
	s, err := this.serverConn.NewSession()
	if err != nil {
		return nil, err
	}
	stdinPipe, err := s.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdoutPipe, err := s.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderrPipe, err := s.StderrPipe()
	if err != nil {
		return nil, err
	}
	return &RemoteCmd{
		stdinPipe,
		stdoutPipe,
		stderrPipe,
		cmd,
		s,
	}, nil
}

func (this *LocalCmd) Run() ([]string, error) {
	out := make([]string, 0)
	if err := this.Start(); err != nil {
		return nil, err
	}
	stdout := this.StdoutPipe()
	bOut, err := ioutil.ReadAll(stdout)
	if err != nil {
		return nil, err
	}
	stderr := this.StderrPipe()
	bErr, err := ioutil.ReadAll(stderr)
	if err != nil {
		return nil, err
	}
	if err := this.Wait(); err != nil {
		if len(bErr) > 0 {
			return nil, errors.New(err.Error() + "\n" + string(bErr))
		}
		return nil, err
	}
	if len(bOut) > 0 {
		out = append(out, strings.Split(strings.Trim(string(bOut), "\n"), "\n")...)
	}
	if len(bErr) > 0 {
		out = append(out, strings.Split(strings.Trim(string(bErr), "\n"), "\n")...)
	}
	return out, nil
}

func (this *LocalCmd) Start() error {
	return this.cmd.Start()
}

func (this *LocalCmd) Wait() error {
	cerr := this.StderrPipe()
	bErr, err := ioutil.ReadAll(cerr)
	if err != nil {
		return err
	}
	// In this case EOF is not error: http://golang.org/pkg/io/
	// EOF is the error returned by Read when no more input is available.
	// Functions should return EOF only to signal a graceful end of input.
	if err := this.stdinPipe.Close(); err != nil && err != io.EOF {
		if len(bErr) > 0 {
			return errors.New(err.Error() + "\n" + string(bErr))
		}
		return err
	}
	if err := this.cmd.Wait(); err != nil {
		if len(bErr) > 0 {
			return errors.New(err.Error() + "\n" + string(bErr))
		}
		return err
	}
	return nil
}

func (this *LocalCmd) StdinPipe() io.WriteCloser {
	return this.stdinPipe
}

func (this *LocalCmd) StdoutPipe() io.Reader {
	return this.stdoutPipe
}

func (this *LocalCmd) StderrPipe() io.Reader {
	return this.stderrPipe
}

func (this *RemoteCmd) Run() ([]string, error) {
	defer this.session.Close()
	out := make([]string, 0)
	if err := this.Start(); err != nil {
		return nil, err
	}
	stdout := this.StdoutPipe()
	bOut, err := ioutil.ReadAll(stdout)
	if err != nil {
		return nil, err
	}
	stderr := this.StderrPipe()
	bErr, err := ioutil.ReadAll(stderr)
	if err != nil {
		return nil, err
	}
	if err := this.Wait(); err != nil {
		if len(bErr) > 0 {
			return nil, errors.New(err.Error() + "\n" + string(bErr))
		}
		return nil, err
	}
	if len(bOut) > 0 {
		out = append(out, strings.Split(strings.Trim(string(bOut), "\n"), "\n")...)
	}
	if len(bErr) > 0 {
		out = append(out, strings.Split(strings.Trim(string(bErr), "\n"), "\n")...)
	}
	return out, nil
}

func (this *RemoteCmd) Start() error {
	return this.session.Start(this.cmd)
}

func (this *RemoteCmd) Wait() error {
	defer this.session.Close()
	cerr := this.StderrPipe()
	bErr, err := ioutil.ReadAll(cerr)
	if err != nil {
		return err
	}

	// In this case EOF is not error: http://golang.org/pkg/io/
	// EOF is the error returned by Read when no more input is available.
	// Functions should return EOF only to signal a graceful end of input.
	if err := this.stdinPipe.Close(); err != nil && err != io.EOF {
		if len(bErr) > 0 {
			return errors.New(err.Error() + "\n" + string(bErr))
		}
		return err
	}
	if err := this.session.Wait(); err != nil {
		if len(bErr) > 0 {
			return errors.New(err.Error() + "\n" + string(bErr))
		}
		return err
	}
	return nil
}

func (this *RemoteCmd) StdinPipe() io.WriteCloser {
	return this.stdinPipe
}

func (this *RemoteCmd) StdoutPipe() io.Reader {
	return this.stdoutPipe
}

func (this *RemoteCmd) StderrPipe() io.Reader {
	return this.stderrPipe
}

func NewLocalRunner() (*Local, error) {
	return &Local{}, nil
}

func NewRemoteKeyAuthRunner(user, host, key string) (*Remote, error) {
	if _, err := os.Stat(key); os.IsNotExist(err) {
		return nil, err
	}
	bs, err := ioutil.ReadFile(key)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(bs)
	if err != nil {
		return nil, err
	}
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
	}
	server, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, err
	}
	return &Remote{server}, nil
}

func NewRemotePassAuthRunner(user, host, password string) (*Remote, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.Password(password)},
	}
	server, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, err
	}
	return &Remote{server}, nil
}

func (this *Remote) CloseConnection() error {
	return this.serverConn.Close()
}
