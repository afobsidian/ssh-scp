package ui

import (
	"os"
	"strings"
	"testing"

	sshclient "ssh-scp/internal/ssh"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// formatSize
// ---------------------------------------------------------------------------

func TestFormatSizeBytes(t *testing.T) {
	got := formatSize(512)
	if got != "512B" {
		t.Errorf("formatSize(512) = %q, want %q", got, "512B")
	}
}

func TestFormatSizeZero(t *testing.T) {
	got := formatSize(0)
	if got != "0B" {
		t.Errorf("formatSize(0) = %q, want %q", got, "0B")
	}
}

func TestFormatSizeKB(t *testing.T) {
	got := formatSize(1536) // 1.5K
	if !strings.HasSuffix(got, "K") {
		t.Errorf("formatSize(1536) = %q, want K suffix", got)
	}
}

func TestFormatSizeMB(t *testing.T) {
	got := formatSize(1024 * 1024 * 2) // 2MB
	if !strings.HasSuffix(got, "M") {
		t.Errorf("formatSize(2MB) = %q, want M suffix", got)
	}
}

func TestFormatSizeGB(t *testing.T) {
	got := formatSize(1024 * 1024 * 1024 * 3) // 3GB
	if !strings.HasSuffix(got, "G") {
		t.Errorf("formatSize(3GB) = %q, want G suffix", got)
	}
}

func TestFormatSizeExactKB(t *testing.T) {
	got := formatSize(1024)
	if got != "1.0K" {
		t.Errorf("formatSize(1024) = %q, want %q", got, "1.0K")
	}
}

// ---------------------------------------------------------------------------
// truncate
// ---------------------------------------------------------------------------

func TestTruncateShort(t *testing.T) {
	got := truncate("hi", 10)
	if got != "hi" {
		t.Errorf("truncate short = %q, want %q", got, "hi")
	}
}

func TestTruncateExact(t *testing.T) {
	got := truncate("hello", 5)
	if got != "hello" {
		t.Errorf("truncate exact = %q, want %q", got, "hello")
	}
}

func TestTruncateLong(t *testing.T) {
	got := truncate("helloworld!", 5)
	if got != "hell…" {
		t.Errorf("truncate long = %q, want %q", got, "hell…")
	}
}

// ---------------------------------------------------------------------------
// truncatePath
// ---------------------------------------------------------------------------

func TestTruncatePathShort(t *testing.T) {
	got := truncatePath("/home/u", 20)
	if got != "/home/u" {
		t.Errorf("truncatePath short = %q, want %q", got, "/home/u")
	}
}

func TestTruncatePathLong(t *testing.T) {
	p := "/very/long/deep/nested/directory/path/here"
	got := truncatePath(p, 15)
	if !strings.HasPrefix(got, "…") {
		t.Errorf("truncated path should start with …, got %q", got)
	}
	// len counts bytes; the … character is 3 bytes in UTF-8
	runeLen := len([]rune(got))
	if runeLen != 15 {
		t.Errorf("truncated rune length = %d, want 15", runeLen)
	}
}

// ---------------------------------------------------------------------------
// joinRemotePath
// ---------------------------------------------------------------------------

func TestJoinRemotePathNormal(t *testing.T) {
	got := joinRemotePath("/home/user", "file.txt")
	if got != "/home/user/file.txt" {
		t.Errorf("joinRemotePath = %q, want %q", got, "/home/user/file.txt")
	}
}

func TestJoinRemotePathTrailingSlash(t *testing.T) {
	got := joinRemotePath("/home/user/", "file.txt")
	if got != "/home/user/file.txt" {
		t.Errorf("joinRemotePath trailing slash = %q, want %q", got, "/home/user/file.txt")
	}
}

func TestJoinRemotePathRoot(t *testing.T) {
	got := joinRemotePath("/", "file.txt")
	if got != "/file.txt" {
		t.Errorf("joinRemotePath root = %q, want %q", got, "/file.txt")
	}
}

func TestJoinRemotePathEmptyDir(t *testing.T) {
	got := joinRemotePath("", "file.txt")
	if got != "/file.txt" {
		t.Errorf("joinRemotePath empty dir = %q, want %q", got, "/file.txt")
	}
}

// ---------------------------------------------------------------------------
// visibleHeight
// ---------------------------------------------------------------------------

func TestVisibleHeight(t *testing.T) {
	m := FileBrowserModel{height: 30}
	got := m.visibleHeight()
	if got != 22 { // 30 - 8
		t.Errorf("visibleHeight = %d, want 22", got)
	}
}

func TestVisibleHeightMinimum(t *testing.T) {
	m := FileBrowserModel{height: 5}
	got := m.visibleHeight()
	if got < 1 {
		t.Errorf("visibleHeight should be at least 1, got %d", got)
	}
}

func TestVisibleHeightZero(t *testing.T) {
	m := FileBrowserModel{height: 0}
	got := m.visibleHeight()
	if got != 1 {
		t.Errorf("visibleHeight(0) = %d, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - constructor and selection helpers
// ---------------------------------------------------------------------------

func TestNewFileBrowserModelLocalDir(t *testing.T) {
	dir := t.TempDir()
	m := NewFileBrowserModel(nil, dir, "/remote")
	if m.localDir != dir {
		t.Errorf("localDir = %q, want %q", m.localDir, dir)
	}
	if m.remoteDir != "/remote" {
		t.Errorf("remoteDir = %q, want %q", m.remoteDir, "/remote")
	}
}

func TestSelectedLocalFileEmpty(t *testing.T) {
	m := FileBrowserModel{}
	got := m.SelectedLocalFile()
	if got != "" {
		t.Errorf("SelectedLocalFile empty = %q, want empty", got)
	}
}

func TestSelectedRemoteFileEmpty(t *testing.T) {
	m := FileBrowserModel{}
	got := m.SelectedRemoteFile()
	if got != "" {
		t.Errorf("SelectedRemoteFile empty = %q, want empty", got)
	}
}

func TestSelectedRemoteFile(t *testing.T) {
	m := FileBrowserModel{
		remoteDir:    "/home/user",
		remoteCursor: 0,
		remoteFiles: []sshclient.RemoteFile{
			{Name: "test.txt"},
		},
	}
	got := m.SelectedRemoteFile()
	if got != "/home/user/test.txt" {
		t.Errorf("SelectedRemoteFile = %q, want %q", got, "/home/user/test.txt")
	}
}

func TestSetDimensions(t *testing.T) {
	m := FileBrowserModel{}
	m.SetDimensions(100, 50)
	if m.width != 100 || m.height != 50 {
		t.Errorf("dimensions = %dx%d, want 100x50", m.width, m.height)
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - Update key handling
// ---------------------------------------------------------------------------

func TestFBUpdateTabSwitchesPanels(t *testing.T) {
	m := FileBrowserModel{focus: panelLocal, width: 80, height: 30}
	msg := tea.KeyMsg{Type: tea.KeyTab}
	m, _ = m.Update(msg)
	if m.focus != panelRemote {
		t.Errorf("after Tab, focus = %d, want panelRemote", m.focus)
	}
	m, _ = m.Update(msg)
	if m.focus != panelLocal {
		t.Errorf("after second Tab, focus = %d, want panelLocal", m.focus)
	}
}

func TestFBUpdateLocalCursorDown(t *testing.T) {
	dir := t.TempDir()
	// Create a few files
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		os.WriteFile(dir+"/"+name, []byte("x"), 0644)
	}
	m := NewFileBrowserModel(nil, dir, "/remote")
	m.height = 30
	m.width = 80

	msg := tea.KeyMsg{Type: tea.KeyDown}
	m, _ = m.Update(msg)
	if m.localCursor != 1 {
		t.Errorf("localCursor = %d, want 1", m.localCursor)
	}
}

func TestFBUpdateLocalCursorUp(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt"} {
		os.WriteFile(dir+"/"+name, []byte("x"), 0644)
	}
	m := NewFileBrowserModel(nil, dir, "/remote")
	m.localCursor = 1
	m.height = 30
	m.width = 80

	msg := tea.KeyMsg{Type: tea.KeyUp}
	m, _ = m.Update(msg)
	if m.localCursor != 0 {
		t.Errorf("localCursor = %d, want 0", m.localCursor)
	}
}

func TestFBUpdateRemoteCursorDown(t *testing.T) {
	m := FileBrowserModel{
		focus:       panelRemote,
		remoteFiles: []sshclient.RemoteFile{{Name: "a"}, {Name: "b"}, {Name: "c"}},
		height:      30,
		width:       80,
	}
	msg := tea.KeyMsg{Type: tea.KeyDown}
	m, _ = m.Update(msg)
	if m.remoteCursor != 1 {
		t.Errorf("remoteCursor = %d, want 1", m.remoteCursor)
	}
}

func TestFBUpdateViewNonZeroWidth(t *testing.T) {
	m := FileBrowserModel{width: 80, height: 30}
	view := m.View()
	if strings.Contains(view, "Loading...") {
		t.Error("View should render panels when width > 0")
	}
}

func TestFBUpdateViewZeroWidth(t *testing.T) {
	m := FileBrowserModel{}
	view := m.View()
	if view != "Loading..." {
		t.Errorf("View with zero width = %q, want %q", view, "Loading...")
	}
}

func TestFBUpdateWindowSizeMsg(t *testing.T) {
	m := FileBrowserModel{}
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.width != 120 || m.height != 40 {
		t.Errorf("after WindowSizeMsg: %dx%d, want 120x40", m.width, m.height)
	}
}

func TestFBUpdateTransferDoneMsgError(t *testing.T) {
	m := FileBrowserModel{
		transferring:     true,
		transferProgress: "file.txt",
	}
	m, _ = m.Update(TransferDoneMsg{Err: os.ErrNotExist})
	if !m.transferring {
		// transferring should be set to false
	}
	if !strings.Contains(m.statusMsg, "Transfer failed") {
		t.Errorf("statusMsg = %q, want Transfer failed", m.statusMsg)
	}
}

func TestFBUpdateRemoteFilesMsg(t *testing.T) {
	m := FileBrowserModel{remoteCursor: 5}
	m, _ = m.Update(remoteFilesMsg{
		files: []sshclient.RemoteFile{{Name: "a.txt"}, {Name: "b.txt"}},
	})
	if len(m.remoteFiles) != 2 {
		t.Fatalf("expected 2 remote files, got %d", len(m.remoteFiles))
	}
	if m.remoteCursor != 0 {
		t.Errorf("remoteCursor should reset to 0 when exceeding count, got %d", m.remoteCursor)
	}
}

func TestFBUpdateRemoteFilesMsgError(t *testing.T) {
	m := FileBrowserModel{}
	m, _ = m.Update(remoteFilesMsg{err: os.ErrPermission})
	if !strings.Contains(m.statusMsg, "Error") {
		t.Errorf("statusMsg = %q, want error message", m.statusMsg)
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - scroll tracking
// ---------------------------------------------------------------------------

func TestFBScrollDownLocal(t *testing.T) {
	dir := t.TempDir()
	// Create more files than visible
	for i := 0; i < 30; i++ {
		name := strings.Repeat("f", i+1) + ".txt"
		os.WriteFile(dir+"/"+name, []byte("x"), 0644)
	}
	m := NewFileBrowserModel(nil, dir, "/remote")
	m.height = 15 // small height to trigger scroll

	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	for i := 0; i < 20; i++ {
		m, _ = m.Update(downMsg)
	}
	if m.localScroll == 0 {
		t.Error("localScroll should have advanced")
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - local directory navigation
// ---------------------------------------------------------------------------

func TestFBLocalEnterDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir+"/subdir", 0755)
	os.WriteFile(dir+"/subdir/inner.txt", []byte("x"), 0644)

	m := NewFileBrowserModel(nil, dir, "/remote")
	m.height = 30
	m.width = 80

	// subdir should be in the list; find its index
	found := false
	for i, f := range m.localFiles {
		if f.Name() == "subdir" {
			m.localCursor = i
			found = true
			break
		}
	}
	if !found {
		t.Fatal("subdir not found in localFiles")
	}

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	m, _ = m.Update(enterMsg)
	if !strings.HasSuffix(m.localDir, "subdir") {
		t.Errorf("localDir = %q, should end with subdir", m.localDir)
	}
}

func TestFBLocalBackspace(t *testing.T) {
	dir := t.TempDir()
	sub := dir + "/sub"
	os.MkdirAll(sub, 0755)

	m := NewFileBrowserModel(nil, sub, "/remote")
	m.height = 30
	m.width = 80

	bsMsg := tea.KeyMsg{Type: tea.KeyBackspace}
	m, _ = m.Update(bsMsg)
	if m.localDir != dir {
		t.Errorf("localDir = %q, want %q", m.localDir, dir)
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - remote cursor & scroll
// ---------------------------------------------------------------------------

func TestFBRemoteCursorUp(t *testing.T) {
	m := FileBrowserModel{
		focus:        panelRemote,
		remoteCursor: 2,
		remoteFiles:  []sshclient.RemoteFile{{Name: "a"}, {Name: "b"}, {Name: "c"}},
		height:       30,
	}
	msg := tea.KeyMsg{Type: tea.KeyUp}
	m, _ = m.Update(msg)
	if m.remoteCursor != 1 {
		t.Errorf("remoteCursor = %d, want 1", m.remoteCursor)
	}
}

func TestFBRemoteScrollUp(t *testing.T) {
	files := make([]sshclient.RemoteFile, 30)
	for i := range files {
		files[i] = sshclient.RemoteFile{Name: strings.Repeat("r", i+1)}
	}
	m := FileBrowserModel{
		focus:        panelRemote,
		remoteCursor: 15,
		remoteScroll: 10,
		remoteFiles:  files,
		height:       15,
	}
	// Scroll up past the top of the viewport
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	for i := 0; i < 10; i++ {
		m, _ = m.Update(upMsg)
	}
	if m.remoteScroll > m.remoteCursor {
		t.Errorf("remoteScroll=%d should not exceed remoteCursor=%d", m.remoteScroll, m.remoteCursor)
	}
}

func TestFBRemoteScrollDown(t *testing.T) {
	files := make([]sshclient.RemoteFile, 30)
	for i := range files {
		files[i] = sshclient.RemoteFile{Name: strings.Repeat("r", i+1)}
	}
	m := FileBrowserModel{
		focus:       panelRemote,
		remoteFiles: files,
		height:      15,
	}
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	for i := 0; i < 20; i++ {
		m, _ = m.Update(downMsg)
	}
	if m.remoteScroll == 0 {
		t.Error("remoteScroll should have advanced")
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - remote directory navigation
// ---------------------------------------------------------------------------

func TestFBRemoteEnterDir(t *testing.T) {
	m := FileBrowserModel{
		focus:     panelRemote,
		remoteDir: "/home/user",
		remoteFiles: []sshclient.RemoteFile{
			{Name: "subdir", IsDir: true},
			{Name: "file.txt", IsDir: false},
		},
		remoteCursor: 0,
		height:       30,
	}
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	m, cmd := m.Update(enterMsg)
	if m.remoteDir != "/home/user/subdir" {
		t.Errorf("remoteDir = %q, want /home/user/subdir", m.remoteDir)
	}
	if m.remoteCursor != 0 {
		t.Errorf("remoteCursor should reset to 0")
	}
	if m.remoteScroll != 0 {
		t.Errorf("remoteScroll should reset to 0")
	}
	if cmd == nil {
		t.Error("entering remote dir should return a refresh command")
	}
}

func TestFBRemoteEnterFile(t *testing.T) {
	m := FileBrowserModel{
		focus:     panelRemote,
		remoteDir: "/home/user",
		remoteFiles: []sshclient.RemoteFile{
			{Name: "file.txt", IsDir: false},
		},
		remoteCursor: 0,
		height:       30,
	}
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	m, _ = m.Update(enterMsg)
	// Should not change directory
	if m.remoteDir != "/home/user" {
		t.Errorf("remoteDir should not change for file, got %q", m.remoteDir)
	}
}

func TestFBRemoteBackspace(t *testing.T) {
	m := FileBrowserModel{
		focus:     panelRemote,
		remoteDir: "/home/user/docs",
		height:    30,
	}
	bsMsg := tea.KeyMsg{Type: tea.KeyBackspace}
	m, cmd := m.Update(bsMsg)
	if m.remoteDir != "/home/user" {
		t.Errorf("remoteDir = %q, want /home/user", m.remoteDir)
	}
	if cmd == nil {
		t.Error("backspace on remote should return refresh command")
	}
}

func TestFBRemoteBackspaceAtRoot(t *testing.T) {
	m := FileBrowserModel{
		focus:     panelRemote,
		remoteDir: "/",
		height:    30,
	}
	bsMsg := tea.KeyMsg{Type: tea.KeyBackspace}
	m, _ = m.Update(bsMsg)
	if m.remoteDir != "/" {
		t.Errorf("remoteDir at root should stay /, got %q", m.remoteDir)
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - Enter on local file (non-dir)
// ---------------------------------------------------------------------------

func TestFBLocalEnterFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(dir+"/file.txt", []byte("content"), 0644)

	m := NewFileBrowserModel(nil, dir, "/remote")
	m.height = 30
	m.width = 80

	// file.txt should be at cursor 0
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	oldDir := m.localDir
	m, _ = m.Update(enterMsg)
	if m.localDir != oldDir {
		t.Errorf("localDir should not change for file, got %q", m.localDir)
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - k/j keys
// ---------------------------------------------------------------------------

func TestFBVimKeysDown(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt"} {
		os.WriteFile(dir+"/"+name, []byte("x"), 0644)
	}
	m := NewFileBrowserModel(nil, dir, "/remote")
	m.height = 30

	jMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	m, _ = m.Update(jMsg)
	if m.localCursor != 1 {
		t.Errorf("j should move cursor down, got %d", m.localCursor)
	}
}

func TestFBVimKeysUp(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt"} {
		os.WriteFile(dir+"/"+name, []byte("x"), 0644)
	}
	m := NewFileBrowserModel(nil, dir, "/remote")
	m.localCursor = 1
	m.height = 30

	kMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}
	m, _ = m.Update(kMsg)
	if m.localCursor != 0 {
		t.Errorf("k should move cursor up, got %d", m.localCursor)
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - rendering with files
// ---------------------------------------------------------------------------

func TestFBRenderLocalPanelWithFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(dir+"/readme.txt", []byte("hello world"), 0644)
	os.MkdirAll(dir+"/docs", 0755)

	m := NewFileBrowserModel(nil, dir, "/remote")
	m.width = 80
	m.height = 30

	view := m.renderLocalPanel(40, 20)
	if !strings.Contains(view, "Local") {
		t.Error("should contain 'Local' header")
	}
}

func TestFBRenderRemotePanelWithFiles(t *testing.T) {
	m := FileBrowserModel{
		remoteDir: "/home/user",
		remoteFiles: []sshclient.RemoteFile{
			{Name: "file1.txt", Size: 1024, IsDir: false},
			{Name: "Documents", Size: 4096, IsDir: true},
		},
		width:  80,
		height: 30,
	}

	view := m.renderRemotePanel(40, 20)
	if !strings.Contains(view, "Remote") {
		t.Error("should contain 'Remote' header")
	}
}

func TestFBRenderLocalPanelWithSelection(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(dir+"/a.txt", []byte("a"), 0644)
	os.WriteFile(dir+"/b.txt", []byte("b"), 0644)

	m := NewFileBrowserModel(nil, dir, "/remote")
	m.width = 80
	m.height = 30
	m.localCursor = 1

	view := m.renderLocalPanel(40, 20)
	if view == "" {
		t.Error("view should not be empty")
	}
}

func TestFBRenderRemotePanelWithSelection(t *testing.T) {
	m := FileBrowserModel{
		remoteDir: "/home",
		remoteFiles: []sshclient.RemoteFile{
			{Name: "a.txt", Size: 100},
			{Name: "b.txt", Size: 200},
		},
		remoteCursor: 1,
		width:        80,
		height:       30,
	}

	view := m.renderRemotePanel(40, 20)
	if view == "" {
		t.Error("view should not be empty")
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - View with panels
// ---------------------------------------------------------------------------

func TestFBViewWithFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(dir+"/test.txt", []byte("x"), 0644)

	m := NewFileBrowserModel(nil, dir, "/remote")
	m.width = 80
	m.height = 30
	m.remoteFiles = []sshclient.RemoteFile{{Name: "remote.txt", Size: 512}}

	view := m.View()
	if !strings.Contains(view, "Tab") {
		t.Error("view should contain status bar with Tab hint")
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - SelectedLocalFile with files
// ---------------------------------------------------------------------------

func TestSelectedLocalFileWithFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(dir+"/test.txt", []byte("x"), 0644)

	m := NewFileBrowserModel(nil, dir, "/remote")
	got := m.SelectedLocalFile()
	if got == "" {
		t.Error("SelectedLocalFile should not be empty")
	}
	if !strings.HasSuffix(got, "test.txt") {
		t.Errorf("SelectedLocalFile = %q, should end with test.txt", got)
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - TransferDoneMsg success
// ---------------------------------------------------------------------------

func TestFBTransferDoneSuccess(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(dir+"/test.txt", []byte("x"), 0644)

	m := NewFileBrowserModel(nil, dir, "/remote")
	m.transferring = true
	m.transferProgress = "test.txt"
	m.height = 30

	m, _ = m.Update(TransferDoneMsg{Err: nil})
	if m.transferring {
		t.Error("transferring should be false after success")
	}
	if !strings.Contains(m.statusMsg, "Transfer complete") {
		t.Errorf("statusMsg = %q, want Transfer complete", m.statusMsg)
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - local scroll up tracking
// ---------------------------------------------------------------------------

func TestFBLocalScrollUpTracking(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 30; i++ {
		name := strings.Repeat("f", i+1) + ".txt"
		os.WriteFile(dir+"/"+name, []byte("x"), 0644)
	}
	m := NewFileBrowserModel(nil, dir, "/remote")
	m.height = 15

	// Move down first
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	for i := 0; i < 20; i++ {
		m, _ = m.Update(downMsg)
	}

	// Now move up past viewport top
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	for i := 0; i < 20; i++ {
		m, _ = m.Update(upMsg)
	}
	if m.localScroll > m.localCursor {
		t.Errorf("localScroll=%d should not exceed localCursor=%d", m.localScroll, m.localCursor)
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - cursor at bounds
// ---------------------------------------------------------------------------

func TestFBLocalCursorAtZeroNoUp(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(dir+"/a.txt", []byte("x"), 0644)
	m := NewFileBrowserModel(nil, dir, "/remote")
	m.height = 30

	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	m, _ = m.Update(upMsg)
	if m.localCursor != 0 {
		t.Errorf("cursor should stay at 0, got %d", m.localCursor)
	}
}

func TestFBLocalCursorAtEndNoDown(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(dir+"/a.txt", []byte("x"), 0644)
	m := NewFileBrowserModel(nil, dir, "/remote")
	m.localCursor = len(m.localFiles) - 1
	m.height = 30

	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	m, _ = m.Update(downMsg)
	if m.localCursor != len(m.localFiles)-1 {
		t.Errorf("cursor should stay at end, got %d", m.localCursor)
	}
}

func TestFBRemoteCursorAtZeroNoUp(t *testing.T) {
	m := FileBrowserModel{
		focus:       panelRemote,
		remoteFiles: []sshclient.RemoteFile{{Name: "a"}},
		height:      30,
	}
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	m, _ = m.Update(upMsg)
	if m.remoteCursor != 0 {
		t.Errorf("cursor should stay at 0, got %d", m.remoteCursor)
	}
}

func TestFBRemoteCursorAtEndNoDown(t *testing.T) {
	m := FileBrowserModel{
		focus:        panelRemote,
		remoteFiles:  []sshclient.RemoteFile{{Name: "a"}, {Name: "b"}},
		remoteCursor: 1,
		height:       30,
	}
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	m, _ = m.Update(downMsg)
	if m.remoteCursor != 1 {
		t.Errorf("cursor should stay at 1, got %d", m.remoteCursor)
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - empty file lists
// ---------------------------------------------------------------------------

func TestFBEnterLocalEmptyFiles(t *testing.T) {
	m := FileBrowserModel{focus: panelLocal, localDir: "/tmp", height: 30}
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	m, _ = m.Update(enterMsg)
	// Should not panic with empty localFiles
}

func TestFBEnterRemoteEmptyFiles(t *testing.T) {
	m := FileBrowserModel{focus: panelRemote, remoteDir: "/home", height: 30}
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	m, _ = m.Update(enterMsg)
	// Should not panic with empty remoteFiles
}

// ---------------------------------------------------------------------------
// FileBrowserModel - small panel height
// ---------------------------------------------------------------------------

func TestFBRenderLocalPanelSmallHeight(t *testing.T) {
	m := FileBrowserModel{
		localDir: "/tmp",
		width:    80,
		height:   30,
	}
	view := m.renderLocalPanel(40, 2)
	if view == "" {
		t.Error("should render even at small height")
	}
}

func TestFBRenderRemotePanelSmallHeight(t *testing.T) {
	m := FileBrowserModel{
		remoteDir: "/home",
		width:     80,
		height:    30,
	}
	view := m.renderRemotePanel(40, 2)
	if view == "" {
		t.Error("should render even at small height")
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - View minimal height
// ---------------------------------------------------------------------------

func TestFBViewMinimalHeight(t *testing.T) {
	m := FileBrowserModel{width: 80, height: 5}
	view := m.View()
	if view == "" {
		t.Error("should render at minimal height")
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - Ctrl+U upload (local panel, file selected)
// ---------------------------------------------------------------------------

func TestFBCtrlUUpload(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(dir+"/upload.txt", []byte("data"), 0644)

	m := NewFileBrowserModel(nil, dir, "/remote")
	m.height = 30
	m.width = 80
	m.focus = panelLocal

	msg := tea.KeyMsg{Type: tea.KeyCtrlU}
	m, cmd := m.Update(msg)
	if !m.transferring {
		t.Error("transferring should be true after ctrl+u")
	}
	if m.transferProgress != "upload.txt" {
		t.Errorf("transferProgress = %q, want upload.txt", m.transferProgress)
	}
	if !strings.Contains(m.statusMsg, "Uploading") {
		t.Errorf("statusMsg = %q, want Uploading", m.statusMsg)
	}
	if cmd == nil {
		t.Error("should return transfer command")
	}
}

func TestFBCtrlUOnDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir+"/subdir", 0755)

	m := NewFileBrowserModel(nil, dir, "/remote")
	m.height = 30
	m.width = 80
	m.focus = panelLocal

	// Find dir cursor
	for i, f := range m.localFiles {
		if f.IsDir() {
			m.localCursor = i
			break
		}
	}

	msg := tea.KeyMsg{Type: tea.KeyCtrlU}
	m, cmd := m.Update(msg)
	if m.transferring {
		t.Error("should not transfer directory")
	}
	if cmd != nil {
		t.Error("should not return command for directory")
	}
}

func TestFBCtrlUWhileTransferring(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(dir+"/f.txt", []byte("x"), 0644)

	m := NewFileBrowserModel(nil, dir, "/remote")
	m.height = 30
	m.transferring = true

	msg := tea.KeyMsg{Type: tea.KeyCtrlU}
	m, cmd := m.Update(msg)
	if cmd != nil {
		t.Error("should not start another transfer while transferring")
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - Ctrl+D download (remote panel, file selected)
// ---------------------------------------------------------------------------

func TestFBCtrlDDownload(t *testing.T) {
	dir := t.TempDir()
	m := FileBrowserModel{
		focus:     panelRemote,
		localDir:  dir,
		remoteDir: "/home/user",
		remoteFiles: []sshclient.RemoteFile{
			{Name: "download.txt", IsDir: false, Size: 100},
		},
		remoteCursor: 0,
		height:       30,
	}

	msg := tea.KeyMsg{Type: tea.KeyCtrlD}
	m, cmd := m.Update(msg)
	if !m.transferring {
		t.Error("transferring should be true after ctrl+d")
	}
	if m.transferProgress != "download.txt" {
		t.Errorf("transferProgress = %q, want download.txt", m.transferProgress)
	}
	if !strings.Contains(m.statusMsg, "Downloading") {
		t.Errorf("statusMsg = %q, want Downloading", m.statusMsg)
	}
	if cmd == nil {
		t.Error("should return transfer command")
	}
}

func TestFBCtrlDOnDir(t *testing.T) {
	m := FileBrowserModel{
		focus:     panelRemote,
		remoteDir: "/home",
		remoteFiles: []sshclient.RemoteFile{
			{Name: "subdir", IsDir: true},
		},
		remoteCursor: 0,
		height:       30,
	}

	msg := tea.KeyMsg{Type: tea.KeyCtrlD}
	m, cmd := m.Update(msg)
	if m.transferring {
		t.Error("should not transfer directory")
	}
	if cmd != nil {
		t.Error("should not return command for directory")
	}
}

func TestFBCtrlDWhileTransferring(t *testing.T) {
	m := FileBrowserModel{
		focus:     panelRemote,
		remoteDir: "/home",
		remoteFiles: []sshclient.RemoteFile{
			{Name: "f.txt"},
		},
		transferring: true,
		height:       30,
	}

	msg := tea.KeyMsg{Type: tea.KeyCtrlD}
	m, cmd := m.Update(msg)
	if cmd != nil {
		t.Error("should not start another transfer while transferring")
	}
}

func TestFBCtrlDEmptyFiles(t *testing.T) {
	m := FileBrowserModel{
		focus:     panelRemote,
		remoteDir: "/home",
		height:    30,
	}

	msg := tea.KeyMsg{Type: tea.KeyCtrlD}
	m, cmd := m.Update(msg)
	if cmd != nil {
		t.Error("should not start transfer with empty files")
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - T key (context-aware transfer)
// ---------------------------------------------------------------------------

func TestFBTKeyUploadLocal(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(dir+"/t.txt", []byte("x"), 0644)

	m := NewFileBrowserModel(nil, dir, "/remote")
	m.height = 30
	m.width = 80
	m.focus = panelLocal

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("T")}
	m, cmd := m.Update(msg)
	if !m.transferring {
		t.Error("T on local should start upload")
	}
	if cmd == nil {
		t.Error("should return transfer command")
	}
}

func TestFBTKeyDownloadRemote(t *testing.T) {
	dir := t.TempDir()
	m := FileBrowserModel{
		focus:     panelRemote,
		localDir:  dir,
		remoteDir: "/home",
		remoteFiles: []sshclient.RemoteFile{
			{Name: "r.txt", IsDir: false},
		},
		height: 30,
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("T")}
	m, cmd := m.Update(msg)
	if !m.transferring {
		t.Error("T on remote should start download")
	}
	if cmd == nil {
		t.Error("should return transfer command")
	}
}

func TestFBTKeyOnDirLocal(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir+"/sub", 0755)

	m := NewFileBrowserModel(nil, dir, "/remote")
	m.height = 30
	m.width = 80
	m.focus = panelLocal

	// Find dir
	for i, f := range m.localFiles {
		if f.IsDir() {
			m.localCursor = i
			break
		}
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("T")}
	m, cmd := m.Update(msg)
	if m.transferring {
		t.Error("T on local dir should not start transfer")
	}
	if cmd != nil {
		t.Error("should not return command")
	}
}

func TestFBTKeyOnDirRemote(t *testing.T) {
	m := FileBrowserModel{
		focus:     panelRemote,
		remoteDir: "/home",
		remoteFiles: []sshclient.RemoteFile{
			{Name: "subdir", IsDir: true},
		},
		height: 30,
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("T")}
	m, cmd := m.Update(msg)
	if m.transferring {
		t.Error("T on remote dir should not start transfer")
	}
	if cmd != nil {
		t.Error("should not return command")
	}
}

func TestFBTKeyWhileTransferring(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(dir+"/f.txt", []byte("x"), 0644)

	m := NewFileBrowserModel(nil, dir, "/remote")
	m.transferring = true
	m.height = 30

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("T")}
	m, cmd := m.Update(msg)
	if cmd != nil {
		t.Error("T while transferring should not start new transfer")
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - Init
// ---------------------------------------------------------------------------

func TestFBInit(t *testing.T) {
	// Init requires a client for refreshRemoteCmd, test with nil client
	m := FileBrowserModel{client: nil, remoteDir: "/home"}
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a command")
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - refreshLocal with bad dir
// ---------------------------------------------------------------------------

func TestFBRefreshLocalBadDir(t *testing.T) {
	m := NewFileBrowserModel(nil, "/nonexistent/path/1234", "/remote")
	if len(m.localFiles) != 0 {
		t.Errorf("bad dir should have 0 files, got %d", len(m.localFiles))
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - Ctrl+U with empty local files
// ---------------------------------------------------------------------------

func TestFBCtrlUEmptyFiles(t *testing.T) {
	m := FileBrowserModel{
		focus:    panelLocal,
		localDir: "/nonexistent",
		height:   30,
	}
	msg := tea.KeyMsg{Type: tea.KeyCtrlU}
	m, cmd := m.Update(msg)
	if cmd != nil {
		t.Error("should not start transfer with empty files")
	}
}

// ---------------------------------------------------------------------------
// FileBrowserModel - T with empty files
// ---------------------------------------------------------------------------

func TestFBTKeyEmptyLocalFiles(t *testing.T) {
	m := FileBrowserModel{
		focus:    panelLocal,
		localDir: "/nonexistent",
		height:   30,
	}
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("T")}
	m, cmd := m.Update(msg)
	if cmd != nil {
		t.Error("T with no files should not start transfer")
	}
}

func TestFBTKeyEmptyRemoteFiles(t *testing.T) {
	m := FileBrowserModel{
		focus:     panelRemote,
		remoteDir: "/home",
		height:    30,
	}
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("T")}
	m, cmd := m.Update(msg)
	if cmd != nil {
		t.Error("T with no remote files should not start transfer")
	}
}
