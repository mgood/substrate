package dockerprovisioner

type State string

const CreatedState State = "created"
const RunningState State = "running"
const ReadyState State = "ready"
const PausedState State = "paused"
const RestartingState State = "restarting"
const RemovingState State = "removing"
const ExitedState State = "exited"
const DeadState State = "dead"

func StateFromDockerStatus(s string, ready bool) State {
	if s == string(RunningState) && ready {
		return ReadyState
	}
	return State(s)
}

func (s State) String() string {
	return string(s)
}

func (s State) IsReady() bool {
	return s == ReadyState
}

func (s State) IsPending() bool {
	switch s {
	case CreatedState, RestartingState, RunningState:
		return true
	}

	return false
}

func (s State) IsGone() bool {
	switch s {
	case CreatedState, RunningState, PausedState, RestartingState, RemovingState, ExitedState, DeadState:
		return true
	}

	return false
}
