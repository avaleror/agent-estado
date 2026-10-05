package pass

type exitErr struct {
	code int
	msg  string
}

func (e *exitErr) Error() string { return e.msg }

func exitCode(code int, msg string) error { return &exitErr{code: code, msg: msg} }

func errRejected() error {
	return exitCode(2, "that id was rejected.")
}

func errGone() error {
	return exitCode(2, "no instance with that id. It may have exited.")
}

func errUnreachable() error {
	return exitCode(3, "could not reach that instance. If you have a route, retry with --direct.")
}

func errStopped() error {
	return exitCode(1, "the transfer stopped. Nothing was written.")
}

func errDeclined() error { return exitCode(0, "declined") }

func errVersion() error {
	return exitCode(2, "that instance speaks a different version.")
}

func errBusy() error { return exitCode(2, "that instance is busy.") }

func errNoAnswer() error { return exitCode(2, "No answer. Nothing was sent.") }

func errTooBig(name string) error { return exitCode(1, name+" is too big.") }
