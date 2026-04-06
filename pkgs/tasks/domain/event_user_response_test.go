package domain

import "testing"

func TestEventTypeAcceptsUserResponse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   EventType
		want bool
	}{
		{
			name: "approval requested accepts response",
			in:   EventApprovalRequested,
			want: true,
		},
		{
			name: "task failed accepts response",
			in:   EventTaskFailed,
			want: true,
		},
		{
			name: "task created does not accept response",
			in:   EventTaskCreated,
			want: false,
		},
		{
			name: "unknown event does not accept response",
			in:   EventType("unknown_event"),
			want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := EventTypeAcceptsUserResponse(tc.in); got != tc.want {
				t.Fatalf("EventTypeAcceptsUserResponse(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
