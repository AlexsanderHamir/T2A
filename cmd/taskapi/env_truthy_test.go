package main

import (
	"testing"
)

func TestEnvTruthy(t *testing.T) {
	t.Setenv("T2A_ENV_TRUTHY_TEST", "")
	if envTruthy("T2A_ENV_TRUTHY_TEST") {
		t.Fatal("empty should be false")
	}
	for _, v := range []string{"1", "true", "TRUE", "yes", "Yes", "on", "ON"} {
		t.Run(v, func(t *testing.T) {
			t.Setenv("T2A_ENV_TRUTHY_TEST", v)
			if !envTruthy("T2A_ENV_TRUTHY_TEST") {
				t.Fatalf("want true for %q", v)
			}
		})
	}
	t.Setenv("T2A_ENV_TRUTHY_TEST", "0")
	if envTruthy("T2A_ENV_TRUTHY_TEST") {
		t.Fatal("0 should be false")
	}
}

func TestTaskAPILoggingMinimized_flagWins(t *testing.T) {
	t.Setenv(disableLoggingEnv, "")
	if !taskAPILoggingMinimized(true) {
		t.Fatal("flag true should minimize")
	}
}

func TestTaskAPILoggingMinimized_env(t *testing.T) {
	t.Setenv(disableLoggingEnv, "1")
	if !taskAPILoggingMinimized(false) {
		t.Fatal("env should minimize")
	}
	t.Setenv(disableLoggingEnv, "")
	if taskAPILoggingMinimized(false) {
		t.Fatal("unset env should not minimize")
	}
}
