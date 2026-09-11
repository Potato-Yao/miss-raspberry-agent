package napcat

import (
	"slices"
	"sort"
	"sync"

	"github.com/tidwall/gjson"
)

// MemberDirectory is the reverse index of which of the bot's groups each member
// belongs to. It is populated at startup and refreshed periodically. Storing the
// relation member-first lets callers answer "which shared groups does this user
// belong to?" without scanning every group.
type MemberDirectory struct {
	mu           sync.RWMutex
	friends      map[int64]struct{}
	memberGroups map[int64][]int64
	loaded       bool
}

// NewMemberDirectory creates an empty directory.
func NewMemberDirectory() *MemberDirectory {
	return &MemberDirectory{
		friends:      make(map[int64]struct{}),
		memberGroups: make(map[int64][]int64),
	}
}

// Replace atomically swaps the directory contents with the friend list and the
// member -> group index, and marks it loaded.
func (d *MemberDirectory) Replace(friends []int64, memberGroups map[int64][]int64) {
	if memberGroups == nil {
		memberGroups = make(map[int64][]int64)
	}
	friendSet := make(map[int64]struct{}, len(friends))
	for _, userID := range friends {
		friendSet[userID] = struct{}{}
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.friends = friendSet
	d.memberGroups = memberGroups
	d.loaded = true
}

// IsFriend reports whether userID is one of the bot's friends.
func (d *MemberDirectory) IsFriend(userID int64) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, ok := d.friends[userID]
	return ok
}

// Groups returns a copy of the group IDs that both the bot and userID belong to,
// sorted in ascending order. It returns nil when userID is in no shared group.
func (d *MemberDirectory) Groups(userID int64) []int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	groups := d.memberGroups[userID]
	if len(groups) == 0 {
		return nil
	}
	out := make([]int64, len(groups))
	copy(out, groups)
	slices.Sort(out)
	return out
}

// MemberCount reports the number of distinct members in the directory.
func (d *MemberDirectory) MemberCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.memberGroups)
}

// EdgeCount reports the total number of member-to-group associations.
func (d *MemberDirectory) EdgeCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n := 0
	for _, groups := range d.memberGroups {
		n += len(groups)
	}
	return n
}

// FriendCount reports the number of friends in the directory.
func (d *MemberDirectory) FriendCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.friends)
}

// Loaded reports whether the directory has been populated at least once.
func (d *MemberDirectory) Loaded() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.loaded
}

// BuildMemberGroups converts per-group member lists (group ID -> member IDs) into
// the reverse member -> group IDs index, deduplicating and sorting each entry.
func BuildMemberGroups(groupMembers map[int64][]int64) map[int64][]int64 {
	sets := make(map[int64]map[int64]struct{})
	for groupID, members := range groupMembers {
		for _, userID := range members {
			set := sets[userID]
			if set == nil {
				set = make(map[int64]struct{})
				sets[userID] = set
			}
			set[groupID] = struct{}{}
		}
	}

	out := make(map[int64][]int64, len(sets))
	for userID, set := range sets {
		groups := make([]int64, 0, len(set))
		for groupID := range set {
			groups = append(groups, groupID)
		}
		sort.Slice(groups, func(i, j int) bool { return groups[i] < groups[j] })
		out[userID] = groups
	}
	return out
}

// ParseGroupList extracts group IDs from a get_group_list response data field.
func ParseGroupList(data gjson.Result) []int64 {
	ids := make([]int64, 0, len(data.Array()))
	for _, item := range data.Array() {
		ids = append(ids, item.Get("group_id").Int())
	}
	return ids
}

// ParseGroupMemberList extracts user IDs from a get_group_member_list response data field.
func ParseGroupMemberList(data gjson.Result) []int64 {
	ids := make([]int64, 0, len(data.Array()))
	for _, item := range data.Array() {
		ids = append(ids, item.Get("user_id").Int())
	}
	return ids
}

// ParseFriendList extracts user IDs from a get_friend_list response data field.
func ParseFriendList(data gjson.Result) []int64 {
	ids := make([]int64, 0, len(data.Array()))
	for _, item := range data.Array() {
		ids = append(ids, item.Get("user_id").Int())
	}
	return ids
}
