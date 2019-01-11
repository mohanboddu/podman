package libpod

import (
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/containers/libpod/libpod/lock"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestContainer(id, name string, manager lock.Manager) (*Container, error) {
	ctr := &Container{
		config: &ContainerConfig{
			ID:              id,
			Name:            name,
			RootfsImageID:   id,
			RootfsImageName: "testimg",
			ImageVolumes:    true,
			StaticDir:       "/does/not/exist/",
			LogPath:         "/does/not/exist/",
			Stdin:           true,
			Labels:          map[string]string{"a": "b", "c": "d"},
			StopSignal:      0,
			StopTimeout:     0,
			CreatedTime:     time.Now(),
			Privileged:      true,
			Mounts:          []string{"/does/not/exist"},
			DNSServer:       []net.IP{net.ParseIP("192.168.1.1"), net.ParseIP("192.168.2.2")},
			DNSSearch:       []string{"example.com", "example.example.com"},
			PortMappings: []ocicni.PortMapping{
				{
					HostPort:      80,
					ContainerPort: 90,
					Protocol:      "tcp",
					HostIP:        "192.168.3.3",
				},
				{
					HostPort:      100,
					ContainerPort: 110,
					Protocol:      "udp",
					HostIP:        "192.168.4.4",
				},
			},
		},
		state: &containerState{
			State:      ContainerStateRunning,
			ConfigPath: "/does/not/exist/specs/" + id,
			RunDir:     "/does/not/exist/tmp/",
			Mounted:    true,
			Mountpoint: "/does/not/exist/tmp/" + id,
			PID:        1234,
			ExecSessions: map[string]*ExecSession{
				"abcd": {
					ID:      "1",
					Command: []string{"2", "3"},
					PID:     9876,
				},
				"ef01": {
					ID:      "5",
					Command: []string{"hello", "world"},
					PID:     46765,
				},
			},
			BindMounts: map[string]string{
				"/1/2/3":          "/4/5/6",
				"/test/file.test": "/test2/file2.test",
			},
		},
		runtime: &Runtime{
			config: &RuntimeConfig{
				VolumePath: "/does/not/exist/tmp/volumes",
			},
		},
		valid: true,
	}

	g, err := generate.New("linux")
	if err != nil {
		return nil, err
	}
	ctr.config.Spec = g.Config

	ctr.config.Labels["test"] = "testing"

	// Allocate a lock for the container
	lock, err := manager.AllocateLock()
	if err != nil {
		return nil, err
	}
	ctr.lock = lock
	ctr.config.LockID = lock.ID()

	return ctr, nil
}

func getTestPod(id, name string, manager lock.Manager) (*Pod, error) {
	pod := &Pod{
		config: &PodConfig{
			ID:           id,
			Name:         name,
			Labels:       map[string]string{"a": "b", "c": "d"},
			CgroupParent: "/hello/world/cgroup/parent",
		},
		state: &podState{
			CgroupPath: "/path/to/cgroups/hello/",
		},
		valid: true,
	}

	// Allocate a lock for the pod
	lock, err := manager.AllocateLock()
	if err != nil {
		return nil, err
	}
	pod.lock = lock
	pod.config.LockID = lock.ID()

	return pod, nil
}

func getTestCtrN(n string, manager lock.Manager) (*Container, error) {
	return getTestContainer(strings.Repeat(n, 32), "test"+n, manager)
}

func getTestCtr1(manager lock.Manager) (*Container, error) {
	return getTestCtrN("1", manager)
}

func getTestCtr2(manager lock.Manager) (*Container, error) {
	return getTestCtrN("2", manager)
}

func getTestPodN(n string, manager lock.Manager) (*Pod, error) {
	return getTestPod(strings.Repeat(n, 32), "test"+n, manager)
}

func getTestPod1(manager lock.Manager) (*Pod, error) {
	return getTestPodN("1", manager)
}

func getTestPod2(manager lock.Manager) (*Pod, error) {
	return getTestPodN("2", manager)
}

// This horrible hack tests if containers are equal in a way that should handle
// empty arrays being dropped to nil pointers in the spec JSON
// For some operations (container retrieval from the database) state is allowed
// to be empty. This is controlled by the allowedEmpty bool.
func testContainersEqual(t *testing.T, a, b *Container, allowedEmpty bool) {
	if a == nil && b == nil {
		return
	}
	require.NotNil(t, a)
	require.NotNil(t, b)

	require.NotNil(t, a.config)
	require.NotNil(t, b.config)
	require.NotNil(t, a.state)
	require.NotNil(t, b.state)

	aConfig := new(ContainerConfig)
	bConfig := new(ContainerConfig)
	aState := new(containerState)
	bState := new(containerState)

	blankState := new(containerState)

	assert.Equal(t, a.valid, b.valid)

	assert.Equal(t, a.lock.ID(), b.lock.ID())

	aConfigJSON, err := json.Marshal(a.config)
	assert.NoError(t, err)
	err = json.Unmarshal(aConfigJSON, aConfig)
	assert.NoError(t, err)

	bConfigJSON, err := json.Marshal(b.config)
	assert.NoError(t, err)
	err = json.Unmarshal(bConfigJSON, bConfig)
	assert.NoError(t, err)

	assert.EqualValues(t, aConfig, bConfig)

	aStateJSON, err := json.Marshal(a.state)
	assert.NoError(t, err)
	err = json.Unmarshal(aStateJSON, aState)
	assert.NoError(t, err)

	bStateJSON, err := json.Marshal(b.state)
	assert.NoError(t, err)
	err = json.Unmarshal(bStateJSON, bState)
	assert.NoError(t, err)

	if allowedEmpty {
		assert.True(t, reflect.DeepEqual(aState, bState) || reflect.DeepEqual(aState, blankState))
	} else {
		assert.EqualValues(t, aState, bState)
	}
}

// Test if pods are equal.
// For some operations (pod retrieval from the database) state is allowed to be
// empty. This is controlled by the allowedEmpty bool.
func testPodsEqual(t *testing.T, a, b *Pod, allowedEmpty bool) {
	if a == nil && b == nil {
		return
	}

	blankState := new(podState)

	require.NotNil(t, a)
	require.NotNil(t, b)

	require.NotNil(t, a.config)
	require.NotNil(t, b.config)
	require.NotNil(t, a.state)
	require.NotNil(t, b.state)

	assert.Equal(t, a.valid, b.valid)

	assert.Equal(t, a.lock.ID(), b.lock.ID())

	assert.EqualValues(t, a.config, b.config)

	if allowedEmpty {
		assert.True(t, reflect.DeepEqual(a.state, b.state) || reflect.DeepEqual(a.state, blankState))
	} else {
		assert.EqualValues(t, a.state, b.state)
	}
}
