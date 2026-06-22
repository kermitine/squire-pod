package productivity

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/fforchino/vector-go-sdk/pkg/vectorpb"
)

type fakeBehaviorControlStream struct {
	sent      []*vectorpb.BehaviorControlRequest
	responses []*vectorpb.BehaviorControlResponse
}

func (f *fakeBehaviorControlStream) Send(req *vectorpb.BehaviorControlRequest) error {
	f.sent = append(f.sent, req)
	return nil
}

func (f *fakeBehaviorControlStream) Recv() (*vectorpb.BehaviorControlResponse, error) {
	if len(f.responses) == 0 {
		return nil, errors.New("no response")
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

func TestAcquireBehaviorControlWaitsForGrant(t *testing.T) {
	stream := &fakeBehaviorControlStream{responses: []*vectorpb.BehaviorControlResponse{
		{ResponseType: &vectorpb.BehaviorControlResponse_KeepAlive{KeepAlive: &vectorpb.KeepAlivePing{}}},
		{ResponseType: &vectorpb.BehaviorControlResponse_ControlGrantedResponse{ControlGrantedResponse: &vectorpb.ControlGrantedResponse{}}},
	}}

	if err := acquireBehaviorControl(stream); err != nil {
		t.Fatalf("acquireBehaviorControl() error = %v", err)
	}
	if len(stream.sent) != 1 {
		t.Fatalf("sent %d requests, want 1", len(stream.sent))
	}
	request := stream.sent[0].GetControlRequest()
	if request == nil || request.Priority != vectorpb.ControlRequest_OVERRIDE_BEHAVIORS {
		t.Fatalf("sent control request = %#v", request)
	}
}

func TestAcquireBehaviorControlRejectsLostControl(t *testing.T) {
	stream := &fakeBehaviorControlStream{responses: []*vectorpb.BehaviorControlResponse{
		{ResponseType: &vectorpb.BehaviorControlResponse_ControlLostEvent{ControlLostEvent: &vectorpb.ControlLostResponse{}}},
	}}

	if err := acquireBehaviorControl(stream); err == nil {
		t.Fatal("acquireBehaviorControl() error = nil, want control-lost error")
	}
}

func TestReleaseBehaviorControl(t *testing.T) {
	stream := &fakeBehaviorControlStream{}
	if err := releaseBehaviorControl(stream); err != nil {
		t.Fatalf("releaseBehaviorControl() error = %v", err)
	}
	if len(stream.sent) != 1 || stream.sent[0].GetControlRelease() == nil {
		t.Fatalf("sent requests = %#v, want one control release", stream.sent)
	}
}

func TestClassifyConfirmationIntent(t *testing.T) {
	tests := []struct {
		name   string
		intent string
		want   confirmationResult
	}{
		{name: "affirmative", intent: `{"intent":"intent_imperative_affirmative"}`, want: confirmationAccepted},
		{name: "global yes", intent: `{"intent":"intent_global_yes"}`, want: confirmationAccepted},
		{name: "negative", intent: `{"intent":"intent_imperative_negative"}`, want: confirmationDeclined},
		{name: "unrelated", intent: `{"intent":"intent_greeting_hello"}`, want: confirmationTimedOut},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyConfirmationIntent(tt.intent); got != tt.want {
				t.Fatalf("classifyConfirmationIntent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchesPartialAffirmativeKeyphrase(t *testing.T) {
	tests := []struct {
		name      string
		voiceText string
		keyphrase string
		wantMatch bool
	}{
		{name: "ok", voiceText: "ok", keyphrase: "ok", wantMatch: true},
		{name: "okay", voiceText: "okay that works", keyphrase: "okay", wantMatch: true},
		{name: "sounds good", voiceText: "that sounds good", keyphrase: "sounds good", wantMatch: true},
		{name: "short word boundary", voiceText: "look over there", keyphrase: "ok", wantMatch: false},
		{name: "negated okay", voiceText: "not okay", keyphrase: "okay", wantMatch: false},
		{name: "negative prefix", voiceText: "no, that sounds good", keyphrase: "sounds good", wantMatch: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesPartialIntentKeyphrase("intent_imperative_affirmative", tt.voiceText, tt.keyphrase)
			if got != tt.wantMatch {
				t.Fatalf("matchesPartialIntentKeyphrase() = %v, want %v", got, tt.wantMatch)
			}
		})
	}
}

func TestNotifyConfigUpdatedInvalidatesExistingTasks(t *testing.T) {
	previousGeneration := currentConfigurationGeneration()
	defer func() {
		atomic.StoreUint64(&configurationGeneration, previousGeneration)
		select {
		case <-schedulerRefresh:
		default:
		}
	}()

	task := Task{configurationGeneration: previousGeneration}
	if !taskIsCurrent(task) {
		t.Fatal("new task should match the current configuration")
	}

	NotifyConfigUpdated()
	if taskIsCurrent(task) {
		t.Fatal("task from the previous configuration was not invalidated")
	}
}

func TestTestReminderDoesNotSnooze(t *testing.T) {
	snoozeTask(Task{
		Source:                  "test",
		configurationGeneration: currentConfigurationGeneration(),
	})

	select {
	case task := <-taskQueue:
		t.Fatalf("test reminder was requeued: %#v", task)
	default:
	}
}

func TestWaitForReminderImageHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if waitForReminderImage(ctx) {
		t.Fatal("waitForReminderImage() = true for canceled context")
	}
}

func TestPersonApproachTargetStopsShortOfFace(t *testing.T) {
	robotPose := &vectorpb.PoseStruct{X: 0, Y: 0, OriginId: 7}
	facePose := &vectorpb.PoseStruct{X: 1000, Y: 0, OriginId: 7}

	x, y, heading, shouldMove := personApproachTarget(robotPose, facePose)
	if !shouldMove {
		t.Fatal("personApproachTarget() should move")
	}
	if x != 750 || y != 0 || heading != 0 {
		t.Fatalf("personApproachTarget() = (%v, %v, %v), want (750, 0, 0)", x, y, heading)
	}
}

func TestPersonApproachTargetCapsTravel(t *testing.T) {
	robotPose := &vectorpb.PoseStruct{X: 100, Y: 100, OriginId: 7}
	facePose := &vectorpb.PoseStruct{X: 2100, Y: 100, OriginId: 7}

	x, y, _, shouldMove := personApproachTarget(robotPose, facePose)
	if !shouldMove || x != 1100 || y != 100 {
		t.Fatalf("personApproachTarget() = (%v, %v, move=%v), want (1100, 100, true)", x, y, shouldMove)
	}
}

func TestPersonApproachTargetRejectsUnsafePose(t *testing.T) {
	tests := []struct {
		name  string
		robot *vectorpb.PoseStruct
		face  *vectorpb.PoseStruct
	}{
		{name: "missing pose", robot: nil, face: &vectorpb.PoseStruct{OriginId: 1}},
		{name: "unknown origin", robot: &vectorpb.PoseStruct{}, face: &vectorpb.PoseStruct{}},
		{name: "different origins", robot: &vectorpb.PoseStruct{OriginId: 1}, face: &vectorpb.PoseStruct{X: 500, OriginId: 2}},
		{name: "already close", robot: &vectorpb.PoseStruct{OriginId: 1}, face: &vectorpb.PoseStruct{X: 200, OriginId: 1}},
		{name: "implausibly far", robot: &vectorpb.PoseStruct{OriginId: 1}, face: &vectorpb.PoseStruct{X: 6000, OriginId: 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, _, shouldMove := personApproachTarget(tt.robot, tt.face); shouldMove {
				t.Fatal("personApproachTarget() should reject unsafe pose")
			}
		})
	}
}
