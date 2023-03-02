package memberring

import "github.com/hashicorp/memberlist"

type Role uint8

const (
	RoleUnknown Role = iota
	RoleMember
	RoleWatcher
)

type MemberRing interface {
	Ring

	List() *memberlist.Memberlist
}

type Ring interface {
	GetNode(key []byte) (*memberlist.Node, error)
}

func New(cfg *Config, joinAddrs []string) (MemberRing, error) {
	var ring ring

	cfg.List.Events = &ring
	cfg.List.Delegate = &delegate{Role: cfg.Role}

	list, err := memberlist.Create(&cfg.List)
	if err != nil {
		return nil, err
	}

	_, err = list.Join(joinAddrs)
	if err != nil {
		return nil, err
	}

	if err := ring.init(list.Members()); err != nil {
		return nil, err
	}

	return &memberRing{
		list: list,
		ring: &ring,
	}, nil
}

type memberRing struct {
	ring *ring
	list *memberlist.Memberlist
}

func (m *memberRing) List() *memberlist.Memberlist { return m.list }

func (m *memberRing) GetNode(key []byte) (*memberlist.Node, error) {
	nodeName, err := m.ring.getNodeName(key)
	if err != nil {
		return nil, err
	}

	for _, member := range m.list.Members() {
		if member.Name == nodeName {
			return member, nil
		}
	}

	return nil, errNoNodeFound
}
