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

	d.Replace(map[int64][]int64{7: {200, 100}})
	if !d.Loaded() {
		t.Fatal("directory should be loaded after Replace")
	}
	if d.MemberCount() != 1 || d.EdgeCount() != 2 {
		t.Fatalf("unexpected counts: members=%d edges=%d", d.MemberCount(), d.EdgeCount())
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
	d.Replace(map[int64][]int64{7: {100}})
	d.Replace(map[int64][]int64{8: {200}})
	if d.Groups(7) != nil {
		t.Fatalf("expected old member 7 to be gone, got %v", d.Groups(7))
	}
	if d.MemberCount() != 1 {
		t.Fatalf("MemberCount = %d, want 1", d.MemberCount())
	}
}
