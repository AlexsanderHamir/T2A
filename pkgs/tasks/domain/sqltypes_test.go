package domain

import "testing"

func TestStatus_Scan_string_and_bytes(t *testing.T) {
	var s Status
	if err := s.Scan("ready"); err != nil || s != StatusReady {
		t.Fatalf("string: %v %s", err, s)
	}
	if err := s.Scan([]byte("running")); err != nil || s != StatusRunning {
		t.Fatalf("[]byte: %v %s", err, s)
	}
}

func TestStatus_Scan_nil_zeroes(t *testing.T) {
	s := StatusReady
	if err := s.Scan(nil); err != nil || s != "" {
		t.Fatalf("nil: %v %q", err, s)
	}
}

func TestStatus_Scan_rejects_wrong_type(t *testing.T) {
	var s Status
	if err := s.Scan(42); err == nil {
		t.Fatal("expected error for int")
	}
}

func TestEventType_Scan_and_Value_roundTrip(t *testing.T) {
	var e EventType
	if err := e.Scan("task_created"); err != nil || e != EventTaskCreated {
		t.Fatal(err)
	}
	v, err := e.Value()
	if err != nil || v != string(EventTaskCreated) {
		t.Fatalf("value %v %v", v, err)
	}
}

func TestActor_Scan_string(t *testing.T) {
	var a Actor
	if err := a.Scan("agent"); err != nil || a != ActorAgent {
		t.Fatal(err)
	}
}

func TestPriority_Scan_bytes(t *testing.T) {
	var p Priority
	if err := p.Scan([]byte("high")); err != nil || p != PriorityHigh {
		t.Fatal(err)
	}
}
