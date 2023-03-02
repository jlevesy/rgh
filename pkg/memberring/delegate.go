package memberring

type delegate struct {
	Role Role
}

func (d delegate) NodeMeta(limit int) []byte                   { return []byte{byte(d.Role)} }
func (d *delegate) NotifyMsg([]byte)                           {}
func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte { return nil }
func (d *delegate) LocalState(join bool) []byte                { return nil }
func (d *delegate) MergeRemoteState(buf []byte, join bool)     {}
