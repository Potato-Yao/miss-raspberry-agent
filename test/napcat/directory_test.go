package napcat_test

import (
	"reflect"
	"testing"

	"github.com/tidwall/gjson"

	"miss-raspberry-agent/internal/napcat"
)

func TestParseGroupList(t *testing.T) {
	body := `[{"group_id":100,"group_name":"a"},{"group_id":200,"group_name":"b"}]`
	got := napcat.ParseGroupList(gjson.Get(body, "@this"))
	want := []int64{100, 200}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseGroupList = %v, want %v", got, want)
	}
}

func TestParseGroupMemberList(t *testing.T) {
	body := `[{"user_id":1,"nickname":"a"},{"user_id":2,"nickname":"b"}]`
	got := napcat.ParseGroupMemberList(gjson.Get(body, "@this"))
	want := []int64{1, 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseGroupMemberList = %v, want %v", got, want)
	}
}

func TestParseFriendList(t *testing.T) {
	body := `[{"user_id":10,"nickname":"a"},{"user_id":20,"nickname":"b"}]`
	got := napcat.ParseFriendList(gjson.Get(body, "@this"))
	want := []int64{10, 20}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseFriendList = %v, want %v", got, want)
	}
}

func TestBuildMemberGroupsReversesIndex(t *testing.T) {
	got := napcat.BuildMemberGroups(map[int64][]int64{
		100: {1, 2, 3},
		200: {2, 4},
	})
	want := map[int64][]int64{
		1: {100},
		2: {100, 200},
		3: {100},
		4: {200},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildMemberGroups = %v, want %v", got, want)
	}
}

func TestBuildMemberGroupsDeduplicates(t *testing.T) {
	got := napcat.BuildMemberGroups(map[int64][]int64{
		100: {1, 1},
	})
	if !reflect.DeepEqual(got[1], []int64{100}) {
		t.Fatalf("expected deduplicated [100], got %v", got[1])
	}
}

func TestMemberDirectoryGroups(t *testing.T) {
	d := napcat.NewMemberDirectory()
	if d.Loaded() {
		t.Fatal("new directory should not be loaded")
	}

	d.Replace([]int64{7}, map[int64][]int64{7: {200, 100}})
	if !d.Loaded() {
		t.Fatal("directory should be loaded after Replace")
	}
	if d.MemberCount() != 1 || d.EdgeCount() != 2 {
		t.Fatalf("unexpected counts: members=%d edges=%d", d.MemberCount(), d.EdgeCount())
	}
	if d.FriendCount() != 1 || !d.IsFriend(7) {
		t.Fatalf("expected user 7 to be a friend: count=%d", d.FriendCount())
	}
	if d.IsFriend(999) {
		t.Fatal("user 999 should not be a friend")
	}

	got := d.Groups(7)
	if !reflect.DeepEqual(got, []int64{100, 200}) {
		t.Fatalf("Groups(7) = %v, want [100 200]", got)
	}
	if d.Groups(999) != nil {
		t.Fatalf("Groups for unknown member = %v, want nil", d.Groups(999))
	}

	// Mutating the returned slice must not affect the directory.
	got[0] = 999
	if d.Groups(7)[0] != 100 {
		t.Fatal("Groups returned a slice that aliases internal state")
	}
}

func TestMemberDirectoryReplaceClearsOldEntries(t *testing.T) {
	d := napcat.NewMemberDirectory()
	d.Replace([]int64{7}, map[int64][]int64{7: {100}})
	d.Replace(nil, map[int64][]int64{8: {200}})
	if d.Groups(7) != nil {
		t.Fatalf("expected old member 7 to be gone, got %v", d.Groups(7))
	}
	if d.IsFriend(7) {
		t.Fatal("expected old friend 7 to be gone")
	}
	if d.MemberCount() != 1 {
		t.Fatalf("MemberCount = %d, want 1", d.MemberCount())
	}
}

func TestTrySendToMemberViaGroups(t *testing.T) {
	// The first group fails (e.g. temporary session disabled), the second succeeds.
	var attempted []int64
	groupID, ok := napcat.TrySendToMemberViaGroups([]int64{100, 200, 300}, func(g int64) bool {
		attempted = append(attempted, g)
		return g == 200
	})
	if !ok || groupID != 200 {
		t.Fatalf("expected success via group 200, got group=%d ok=%v", groupID, ok)
	}
	if !reflect.DeepEqual(attempted, []int64{100, 200}) {
		t.Fatalf("expected attempts [100 200], got %v", attempted)
	}
}

func TestTrySendToMemberViaGroupsAllFail(t *testing.T) {
	groupID, ok := napcat.TrySendToMemberViaGroups([]int64{100, 200}, func(int64) bool { return false })
	if ok || groupID != 0 {
		t.Fatalf("expected failure, got group=%d ok=%v", groupID, ok)
	}
}
