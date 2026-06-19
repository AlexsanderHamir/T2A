package directive

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func Skipped() {}

//funclogmeasure:skip category=hot-path reason="too short"
func BadReason() {}

func MissingDirective() {}
