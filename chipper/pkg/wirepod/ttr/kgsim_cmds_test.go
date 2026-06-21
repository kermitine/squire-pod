package wirepod_ttr

import (
	"reflect"
	"testing"
)

func TestGetActionsFromString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []RobotAction
	}{
		{
			name:  "plain speech",
			input: "Hello there.",
			want:  []RobotAction{{Action: ActionSayText, Parameter: "Hello there."}},
		},
		{
			name:  "valid animation with surrounding speech",
			input: "Hello. {{playAnimationWI||happy}} Nice to meet you.",
			want: []RobotAction{
				{Action: ActionSayText, Parameter: "Hello."},
				{Action: ActionPlayAnimationWI, Parameter: "happy"},
				{Action: ActionSayText, Parameter: "Nice to meet you."},
			},
		},
		{
			name:  "malformed animation is skipped",
			input: "{{playAnimationWI}} I am still speaking.",
			want:  []RobotAction{{Action: ActionSayText, Parameter: "I am still speaking."}},
		},
		{
			name:  "malformed animation alone does not panic",
			input: "{{playAnimationWI}}",
			want:  nil,
		},
		{
			name:  "unknown command is skipped",
			input: "{{dance||happy}} Moving on.",
			want:  []RobotAction{{Action: ActionSayText, Parameter: "Moving on."}},
		},
		{
			name:  "missing animation name remains a safe action",
			input: "{{playAnimationWI||}} Done.",
			want: []RobotAction{
				{Action: ActionPlayAnimationWI, Parameter: ""},
				{Action: ActionSayText, Parameter: "Done."},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetActionsFromString(tt.input); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("GetActionsFromString(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}
