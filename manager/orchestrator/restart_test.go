package orchestrator

import (
	"testing"
	"time"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/manager/state"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/docker/swarmkit/protobuf/ptypes"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestOrchestratorRestartOnAny(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore(nil)
	assert.NotNil(t, s)

	orchestrator := NewReplicatedOrchestrator(s)
	defer orchestrator.Stop()

	watch, cancel := state.Watch(s.WatchQueue() /*state.EventCreateTask{}, state.EventUpdateTask{}*/)
	defer cancel()

	// Create a service with two instances specified before the orchestrator is
	// started. This should result in two tasks when the orchestrator
	// starts up.
	err := s.Update(func(tx store.Tx) error {
		j1 := &api.Service{
			ID: "id1",
			Spec: api.ServiceSpec{
				Annotations: api.Annotations{
					Name: "name1",
				},
				Task: api.TaskSpec{
					Runtime: &api.TaskSpec_Container{
						Container: &api.ContainerSpec{},
					},
					Restart: &api.RestartPolicy{
						Condition: api.RestartOnAny,
						Delay:     ptypes.DurationProto(0),
					},
				},
				Mode: &api.ServiceSpec_Replicated{
					Replicated: &api.ReplicatedService{
						Replicas: 2,
					},
				},
			},
		}
		assert.NoError(t, store.CreateService(tx, j1))
		return nil
	})
	assert.NoError(t, err)

	// Start the orchestrator.
	go func() {
		assert.NoError(t, orchestrator.Run(ctx))
	}()

	observedTask1 := watchTaskCreate(t, watch)
	assert.Equal(t, observedTask1.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask1.ServiceAnnotations.Name, "name1")

	observedTask2 := watchTaskCreate(t, watch)
	assert.Equal(t, observedTask2.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask2.ServiceAnnotations.Name, "name1")

	// Fail the first task. Confirm that it gets restarted.
	updatedTask1 := observedTask1.Copy()
	updatedTask1.Status = api.TaskStatus{State: api.TaskStateFailed}
	err = s.Update(func(tx store.Tx) error {
		assert.NoError(t, store.UpdateTask(tx, updatedTask1))
		return nil
	})
	assert.NoError(t, err)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)

	observedTask3 := watchTaskCreate(t, watch)
	assert.Equal(t, observedTask3.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask3.ServiceAnnotations.Name, "name1")

	expectCommit(t, watch)

	observedTask4 := watchTaskUpdate(t, watch)
	assert.Equal(t, observedTask4.DesiredState, api.TaskStateRunning)
	assert.Equal(t, observedTask4.ServiceAnnotations.Name, "name1")

	// Mark the second task as completed. Confirm that it gets restarted.
	updatedTask2 := observedTask2.Copy()
	updatedTask2.Status = api.TaskStatus{State: api.TaskStateCompleted}
	err = s.Update(func(tx store.Tx) error {
		assert.NoError(t, store.UpdateTask(tx, updatedTask2))
		return nil
	})
	assert.NoError(t, err)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)

	observedTask5 := watchTaskCreate(t, watch)
	assert.Equal(t, observedTask5.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask5.ServiceAnnotations.Name, "name1")

	expectCommit(t, watch)

	observedTask6 := watchTaskUpdate(t, watch)
	assert.Equal(t, observedTask6.DesiredState, api.TaskStateRunning)
	assert.Equal(t, observedTask6.ServiceAnnotations.Name, "name1")
}

func TestOrchestratorRestartOnFailure(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore(nil)
	assert.NotNil(t, s)

	orchestrator := NewReplicatedOrchestrator(s)
	defer orchestrator.Stop()

	watch, cancel := state.Watch(s.WatchQueue() /*state.EventCreateTask{}, state.EventUpdateTask{}*/)
	defer cancel()

	// Create a service with two instances specified before the orchestrator is
	// started. This should result in two tasks when the orchestrator
	// starts up.
	err := s.Update(func(tx store.Tx) error {
		j1 := &api.Service{
			ID: "id1",
			Spec: api.ServiceSpec{
				Annotations: api.Annotations{
					Name: "name1",
				},
				Task: api.TaskSpec{
					Runtime: &api.TaskSpec_Container{
						Container: &api.ContainerSpec{},
					},
					Restart: &api.RestartPolicy{
						Condition: api.RestartOnFailure,
						Delay:     ptypes.DurationProto(0),
					},
				},
				Mode: &api.ServiceSpec_Replicated{
					Replicated: &api.ReplicatedService{
						Replicas: 2,
					},
				},
			},
		}
		assert.NoError(t, store.CreateService(tx, j1))
		return nil
	})
	assert.NoError(t, err)

	// Start the orchestrator.
	go func() {
		assert.NoError(t, orchestrator.Run(ctx))
	}()

	observedTask1 := watchTaskCreate(t, watch)
	assert.Equal(t, observedTask1.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask1.ServiceAnnotations.Name, "name1")

	observedTask2 := watchTaskCreate(t, watch)
	assert.Equal(t, observedTask2.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask2.ServiceAnnotations.Name, "name1")

	// Fail the first task. Confirm that it gets restarted.
	updatedTask1 := observedTask1.Copy()
	updatedTask1.Status = api.TaskStatus{State: api.TaskStateFailed}
	err = s.Update(func(tx store.Tx) error {
		assert.NoError(t, store.UpdateTask(tx, updatedTask1))
		return nil
	})
	assert.NoError(t, err)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)

	observedTask3 := watchTaskCreate(t, watch)
	assert.Equal(t, observedTask3.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask3.DesiredState, api.TaskStateReady)
	assert.Equal(t, observedTask3.ServiceAnnotations.Name, "name1")

	expectCommit(t, watch)

	observedTask4 := watchTaskUpdate(t, watch)
	assert.Equal(t, observedTask4.DesiredState, api.TaskStateRunning)
	assert.Equal(t, observedTask4.ServiceAnnotations.Name, "name1")

	// Mark the second task as completed. Confirm that it does not get restarted.
	updatedTask2 := observedTask2.Copy()
	updatedTask2.Status = api.TaskStatus{State: api.TaskStateCompleted}
	err = s.Update(func(tx store.Tx) error {
		assert.NoError(t, store.UpdateTask(tx, updatedTask2))
		return nil
	})
	assert.NoError(t, err)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)
	expectCommit(t, watch)

	select {
	case <-watch:
		t.Fatal("got unexpected event")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestOrchestratorRestartOnNone(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore(nil)
	assert.NotNil(t, s)

	orchestrator := NewReplicatedOrchestrator(s)
	defer orchestrator.Stop()

	watch, cancel := state.Watch(s.WatchQueue() /*state.EventCreateTask{}, state.EventUpdateTask{}*/)
	defer cancel()

	// Create a service with two instances specified before the orchestrator is
	// started. This should result in two tasks when the orchestrator
	// starts up.
	err := s.Update(func(tx store.Tx) error {
		j1 := &api.Service{
			ID: "id1",
			Spec: api.ServiceSpec{
				Annotations: api.Annotations{
					Name: "name1",
				},
				Task: api.TaskSpec{
					Runtime: &api.TaskSpec_Container{
						Container: &api.ContainerSpec{},
					},
					Restart: &api.RestartPolicy{
						Condition: api.RestartOnNone,
					},
				},
				Mode: &api.ServiceSpec_Replicated{
					Replicated: &api.ReplicatedService{
						Replicas: 2,
					},
				},
			},
		}
		assert.NoError(t, store.CreateService(tx, j1))
		return nil
	})
	assert.NoError(t, err)

	// Start the orchestrator.
	go func() {
		assert.NoError(t, orchestrator.Run(ctx))
	}()

	observedTask1 := watchTaskCreate(t, watch)
	assert.Equal(t, observedTask1.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask1.ServiceAnnotations.Name, "name1")

	observedTask2 := watchTaskCreate(t, watch)
	assert.Equal(t, observedTask2.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask2.ServiceAnnotations.Name, "name1")

	// Fail the first task. Confirm that it does not get restarted.
	updatedTask1 := observedTask1.Copy()
	updatedTask1.Status.State = api.TaskStateFailed
	err = s.Update(func(tx store.Tx) error {
		assert.NoError(t, store.UpdateTask(tx, updatedTask1))
		return nil
	})
	assert.NoError(t, err)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)
	expectCommit(t, watch)

	select {
	case <-watch:
		t.Fatal("got unexpected event")
	case <-time.After(100 * time.Millisecond):
	}

	// Mark the second task as completed. Confirm that it does not get restarted.
	updatedTask2 := observedTask2.Copy()
	updatedTask2.Status = api.TaskStatus{State: api.TaskStateCompleted}
	err = s.Update(func(tx store.Tx) error {
		assert.NoError(t, store.UpdateTask(tx, updatedTask2))
		return nil
	})
	assert.NoError(t, err)
	expectTaskUpdate(t, watch)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)
	expectCommit(t, watch)

	select {
	case <-watch:
		t.Fatal("got unexpected event")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestOrchestratorRestartDelay(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore(nil)
	assert.NotNil(t, s)

	orchestrator := NewReplicatedOrchestrator(s)
	defer orchestrator.Stop()

	watch, cancel := state.Watch(s.WatchQueue() /*state.EventCreateTask{}, state.EventUpdateTask{}*/)
	defer cancel()

	// Create a service with two instances specified before the orchestrator is
	// started. This should result in two tasks when the orchestrator
	// starts up.
	err := s.Update(func(tx store.Tx) error {
		j1 := &api.Service{
			ID: "id1",
			Spec: api.ServiceSpec{
				Annotations: api.Annotations{
					Name: "name1",
				},
				Task: api.TaskSpec{
					Runtime: &api.TaskSpec_Container{
						Container: &api.ContainerSpec{},
					},
					Restart: &api.RestartPolicy{
						Condition: api.RestartOnAny,
						Delay:     ptypes.DurationProto(100 * time.Millisecond),
					},
				},
				Mode: &api.ServiceSpec_Replicated{
					Replicated: &api.ReplicatedService{
						Replicas: 2,
					},
				},
			},
		}
		assert.NoError(t, store.CreateService(tx, j1))
		return nil
	})
	assert.NoError(t, err)

	// Start the orchestrator.
	go func() {
		assert.NoError(t, orchestrator.Run(ctx))
	}()

	observedTask1 := watchTaskCreate(t, watch)
	assert.Equal(t, observedTask1.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask1.ServiceAnnotations.Name, "name1")

	observedTask2 := watchTaskCreate(t, watch)
	assert.Equal(t, observedTask2.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask2.ServiceAnnotations.Name, "name1")

	// Fail the first task. Confirm that it gets restarted.
	updatedTask1 := observedTask1.Copy()
	updatedTask1.Status = api.TaskStatus{State: api.TaskStateFailed}
	before := time.Now()
	err = s.Update(func(tx store.Tx) error {
		assert.NoError(t, store.UpdateTask(tx, updatedTask1))
		return nil
	})
	assert.NoError(t, err)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)

	observedTask3 := watchTaskCreate(t, watch)
	expectCommit(t, watch)
	assert.Equal(t, observedTask3.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask3.DesiredState, api.TaskStateReady)
	assert.Equal(t, observedTask3.ServiceAnnotations.Name, "name1")

	observedTask4 := watchTaskUpdate(t, watch)
	after := time.Now()

	// At least 100 ms should have elapsed. Only check the lower bound,
	// because the system may be slow and it could have taken longer.
	if after.Sub(before) < 100*time.Millisecond {
		t.Fatalf("restart delay should have elapsed. Got: %v", after.Sub(before))
	}

	assert.Equal(t, observedTask4.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask4.DesiredState, api.TaskStateRunning)
	assert.Equal(t, observedTask4.ServiceAnnotations.Name, "name1")
}

func TestOrchestratorRestartMaxAttempts(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore(nil)
	assert.NotNil(t, s)

	orchestrator := NewReplicatedOrchestrator(s)
	defer orchestrator.Stop()

	watch, cancel := state.Watch(s.WatchQueue() /*state.EventCreateTask{}, state.EventUpdateTask{}*/)
	defer cancel()

	// Create a service with two instances specified before the orchestrator is
	// started. This should result in two tasks when the orchestrator
	// starts up.
	err := s.Update(func(tx store.Tx) error {
		j1 := &api.Service{
			ID: "id1",
			Spec: api.ServiceSpec{
				Annotations: api.Annotations{
					Name: "name1",
				},
				Mode: &api.ServiceSpec_Replicated{
					Replicated: &api.ReplicatedService{
						Replicas: 2,
					},
				},
				Task: api.TaskSpec{
					Runtime: &api.TaskSpec_Container{
						Container: &api.ContainerSpec{},
					},
					Restart: &api.RestartPolicy{
						Condition:   api.RestartOnAny,
						Delay:       ptypes.DurationProto(100 * time.Millisecond),
						MaxAttempts: 1,
					},
				},
			},
		}
		assert.NoError(t, store.CreateService(tx, j1))
		return nil
	})
	assert.NoError(t, err)

	// Start the orchestrator.
	go func() {
		assert.NoError(t, orchestrator.Run(ctx))
	}()

	observedTask1 := watchTaskCreate(t, watch)
	assert.Equal(t, observedTask1.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask1.ServiceAnnotations.Name, "name1")

	observedTask2 := watchTaskCreate(t, watch)
	assert.Equal(t, observedTask2.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask2.ServiceAnnotations.Name, "name1")

	// Fail the first task. Confirm that it gets restarted.
	updatedTask1 := observedTask1.Copy()
	updatedTask1.Status = api.TaskStatus{State: api.TaskStateFailed}
	before := time.Now()
	err = s.Update(func(tx store.Tx) error {
		assert.NoError(t, store.UpdateTask(tx, updatedTask1))
		return nil
	})
	assert.NoError(t, err)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)

	observedTask3 := watchTaskCreate(t, watch)
	expectCommit(t, watch)
	assert.Equal(t, observedTask3.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask3.DesiredState, api.TaskStateReady)
	assert.Equal(t, observedTask3.ServiceAnnotations.Name, "name1")

	observedTask4 := watchTaskUpdate(t, watch)
	after := time.Now()

	// At least 100 ms should have elapsed. Only check the lower bound,
	// because the system may be slow and it could have taken longer.
	if after.Sub(before) < 100*time.Millisecond {
		t.Fatal("restart delay should have elapsed")
	}

	assert.Equal(t, observedTask4.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask4.DesiredState, api.TaskStateRunning)
	assert.Equal(t, observedTask4.ServiceAnnotations.Name, "name1")

	// Fail the second task. Confirm that it gets restarted.
	updatedTask2 := observedTask2.Copy()
	updatedTask2.Status = api.TaskStatus{State: api.TaskStateFailed}
	before = time.Now()
	err = s.Update(func(tx store.Tx) error {
		assert.NoError(t, store.UpdateTask(tx, updatedTask2))
		return nil
	})
	assert.NoError(t, err)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)

	observedTask5 := watchTaskCreate(t, watch)
	expectCommit(t, watch)
	assert.Equal(t, observedTask5.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask5.DesiredState, api.TaskStateReady)

	observedTask6 := watchTaskUpdate(t, watch) // task gets started after a delay
	expectCommit(t, watch)
	assert.Equal(t, observedTask6.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask6.DesiredState, api.TaskStateRunning)
	assert.Equal(t, observedTask6.ServiceAnnotations.Name, "name1")

	// Fail the first instance again. It should not be restarted.
	updatedTask1 = observedTask3.Copy()
	updatedTask1.Status = api.TaskStatus{State: api.TaskStateFailed}
	err = s.Update(func(tx store.Tx) error {
		assert.NoError(t, store.UpdateTask(tx, updatedTask1))
		return nil
	})
	assert.NoError(t, err)
	expectTaskUpdate(t, watch)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)
	expectCommit(t, watch)

	select {
	case <-watch:
		t.Fatal("got unexpected event")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestOrchestratorRestartWindow(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore(nil)
	assert.NotNil(t, s)

	orchestrator := NewReplicatedOrchestrator(s)
	defer orchestrator.Stop()

	watch, cancel := state.Watch(s.WatchQueue() /*state.EventCreateTask{}, state.EventUpdateTask{}*/)
	defer cancel()

	// Create a service with two instances specified before the orchestrator is
	// started. This should result in two tasks when the orchestrator
	// starts up.
	err := s.Update(func(tx store.Tx) error {
		j1 := &api.Service{
			ID: "id1",
			Spec: api.ServiceSpec{
				Annotations: api.Annotations{
					Name: "name1",
				},
				Mode: &api.ServiceSpec_Replicated{
					Replicated: &api.ReplicatedService{
						Replicas: 2,
					},
				},
				Task: api.TaskSpec{
					Restart: &api.RestartPolicy{
						Condition:   api.RestartOnAny,
						Delay:       ptypes.DurationProto(100 * time.Millisecond),
						MaxAttempts: 1,
						Window:      ptypes.DurationProto(500 * time.Millisecond),
					},
				},
			},
		}
		assert.NoError(t, store.CreateService(tx, j1))
		return nil
	})
	assert.NoError(t, err)

	// Start the orchestrator.
	go func() {
		assert.NoError(t, orchestrator.Run(ctx))
	}()

	observedTask1 := watchTaskCreate(t, watch)
	assert.Equal(t, observedTask1.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask1.ServiceAnnotations.Name, "name1")

	observedTask2 := watchTaskCreate(t, watch)
	assert.Equal(t, observedTask2.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask2.ServiceAnnotations.Name, "name1")

	// Fail the first task. Confirm that it gets restarted.
	updatedTask1 := observedTask1.Copy()
	updatedTask1.Status = api.TaskStatus{State: api.TaskStateFailed}
	before := time.Now()
	err = s.Update(func(tx store.Tx) error {
		assert.NoError(t, store.UpdateTask(tx, updatedTask1))
		return nil
	})
	assert.NoError(t, err)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)

	observedTask3 := watchTaskCreate(t, watch)
	expectCommit(t, watch)
	assert.Equal(t, observedTask3.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask3.DesiredState, api.TaskStateReady)
	assert.Equal(t, observedTask3.ServiceAnnotations.Name, "name1")

	observedTask4 := watchTaskUpdate(t, watch)
	after := time.Now()

	// At least 100 ms should have elapsed. Only check the lower bound,
	// because the system may be slow and it could have taken longer.
	if after.Sub(before) < 100*time.Millisecond {
		t.Fatal("restart delay should have elapsed")
	}

	assert.Equal(t, observedTask4.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask4.DesiredState, api.TaskStateRunning)
	assert.Equal(t, observedTask4.ServiceAnnotations.Name, "name1")

	// Fail the second task. Confirm that it gets restarted.
	updatedTask2 := observedTask2.Copy()
	updatedTask2.Status = api.TaskStatus{State: api.TaskStateFailed}
	before = time.Now()
	err = s.Update(func(tx store.Tx) error {
		assert.NoError(t, store.UpdateTask(tx, updatedTask2))
		return nil
	})
	assert.NoError(t, err)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)

	observedTask5 := watchTaskCreate(t, watch)
	expectCommit(t, watch)
	assert.Equal(t, observedTask5.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask5.DesiredState, api.TaskStateReady)
	assert.Equal(t, observedTask5.ServiceAnnotations.Name, "name1")

	observedTask6 := watchTaskUpdate(t, watch) // task gets started after a delay
	expectCommit(t, watch)
	assert.Equal(t, observedTask6.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask6.DesiredState, api.TaskStateRunning)
	assert.Equal(t, observedTask6.ServiceAnnotations.Name, "name1")

	// Fail the first instance again. It should not be restarted.
	updatedTask1 = observedTask3.Copy()
	updatedTask1.Status = api.TaskStatus{State: api.TaskStateFailed}
	before = time.Now()
	err = s.Update(func(tx store.Tx) error {
		assert.NoError(t, store.UpdateTask(tx, updatedTask1))
		return nil
	})
	assert.NoError(t, err)
	expectTaskUpdate(t, watch)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)
	expectCommit(t, watch)

	select {
	case <-watch:
		t.Fatal("got unexpected event")
	case <-time.After(200 * time.Millisecond):
	}

	time.Sleep(time.Second)

	// Fail the second instance again. It should get restarted because
	// enough time has elapsed since the last restarts.
	updatedTask2 = observedTask5.Copy()
	updatedTask2.Status = api.TaskStatus{State: api.TaskStateFailed}
	before = time.Now()
	err = s.Update(func(tx store.Tx) error {
		assert.NoError(t, store.UpdateTask(tx, updatedTask2))
		return nil
	})
	assert.NoError(t, err)
	expectTaskUpdate(t, watch)
	expectCommit(t, watch)
	expectTaskUpdate(t, watch)

	observedTask7 := watchTaskCreate(t, watch)
	expectCommit(t, watch)
	assert.Equal(t, observedTask7.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask7.DesiredState, api.TaskStateReady)

	observedTask8 := watchTaskUpdate(t, watch)
	after = time.Now()

	// At least 100 ms should have elapsed. Only check the lower bound,
	// because the system may be slow and it could have taken longer.
	if after.Sub(before) < 100*time.Millisecond {
		t.Fatal("restart delay should have elapsed")
	}

	assert.Equal(t, observedTask8.Status.State, api.TaskStateNew)
	assert.Equal(t, observedTask8.DesiredState, api.TaskStateRunning)
	assert.Equal(t, observedTask8.ServiceAnnotations.Name, "name1")
}
