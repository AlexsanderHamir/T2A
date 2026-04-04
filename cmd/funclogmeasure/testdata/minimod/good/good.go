package good

import "log/slog"

func ViaVar() {
	l := slog.Default()
	l.Info("x")
}

func ViaChain() {
	slog.Default().Info("x")
}

func ViaPackage() {
	slog.Info("x")
}
