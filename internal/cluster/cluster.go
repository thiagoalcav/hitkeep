package cluster

import (
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"

	"hitkeep/internal/config"
)

type Manager struct {
	list   *memberlist.Memberlist
	lock   sync.RWMutex
	self   string
	leader string
	peers  map[string]string
}

type eventDelegate struct {
	m *Manager
}

func (d *eventDelegate) NotifyJoin(node *memberlist.Node) {
	slog.Debug("Node joined cluster", "name", node.Name, "addr", node.Address())
	d.m.peers[node.Name] = node.Address()
	d.m.reElectLeader()
}

func (d *eventDelegate) NotifyLeave(node *memberlist.Node) {
	slog.Warn("Node left cluster", "name", node.Name)
	delete(d.m.peers, node.Name)
	d.m.reElectLeader()
}

func (d *eventDelegate) NotifyUpdate(node *memberlist.Node) {}

func NewManager(conf *config.Config) (*Manager, error) {
	m := &Manager{
		self:  conf.NodeName,
		peers: make(map[string]string),
	}

	mlConfig := memberlist.DefaultWANConfig()
	mlConfig.Name = conf.NodeName
	mlConfig.BindAddr = conf.BindAddr
	mlConfig.Events = &eventDelegate{m: m}

	list, err := memberlist.Create(mlConfig)
	if err != nil {
		return nil, err
	}
	m.list = list

	if conf.JoinAddr != "" {
		if _, err := list.Join([]string{conf.JoinAddr}); err != nil {
			return nil, err
		}
	}

	m.peers[conf.NodeName] = list.LocalNode().Address()
	m.reElectLeader()
	return m, nil
}

func (m *Manager) IsLeader() bool {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.leader == m.self
}

func (m *Manager) GetLeaderAddr() string {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.peers[m.leader]
}

func (m *Manager) HasPeers() bool {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.list.NumMembers() > 1
}

func (m *Manager) Shutdown() error {
	return m.list.Leave(1 * time.Second)
}

func (m *Manager) reElectLeader() {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Pre-allocate slice to avoid unnecessary allocations (prealloc)
	members := make([]string, 0, len(m.peers))
	for name := range m.peers {
		members = append(members, name)
	}
	sort.Strings(members)

	if len(members) > 0 && m.leader != members[0] {
		m.leader = members[0]
		slog.Debug("New leader elected", "leader", m.leader)
	}

	if m.leader == m.self {
		slog.Debug("This node is now the LEADER.")
	} else {
		slog.Debug("This node is a FOLLOWER.", "current_leader", m.leader)
	}
}
