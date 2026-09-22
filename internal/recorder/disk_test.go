package recorder

import "testing"

func TestDiskLevelFor(t *testing.T) {
	cases := []struct {
		free uint64
		want DiskLevel
	}{
		{free: 20 << 30, want: DiskOK},
		{free: 5 << 30, want: DiskOK},
		{free: 5<<30 - 1, want: DiskLow},
		{free: 1 << 30, want: DiskLow},
		{free: 1<<30 - 1, want: DiskFull},
		{free: 0, want: DiskFull},
	}
	for _, testCase := range cases {
		if got := DiskLevelFor(testCase.free); got != testCase.want {
			t.Errorf("DiskLevelFor(%d) = %v, want %v", testCase.free, got, testCase.want)
		}
	}
}

func TestFreeSpaceReadsTheTempDirectory(t *testing.T) {
	if !Supported() {
		t.Skip("free space is read on macOS only")
	}
	free, err := FreeSpace(t.TempDir())
	if err != nil || free == 0 {
		t.Errorf("FreeSpace = %d, %v; want a positive number and no error", free, err)
	}
}
