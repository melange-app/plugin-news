package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

type Update struct {
	Version   string
	Changelog string
	Download  string
}

// Environmental Variables Required
//
// MLGUPDATEDIR - the Directory where the Update was Downloaded
// func main() {
// 	updateDir := os.Getenv("MLGUPDATEDIR")
// 	if updateDir == "" {
// 		return
// 	}
// 	err := installUpdate(updateDir)
// 	fmt.Println(err)
// }

type updateStatus struct {
	Msg      string
	NoUpdate bool
	Updated  bool
	err      error
}

func (u updateStatus) HasError() bool {
	return u.err != nil
}

func (u updateStatus) Error() string {
	return fmt.Sprintf("%s %s", u.Msg, u.err)
}

type progressManager struct {
	Writer   io.Writer
	Length   int64
	Progress chan<- float64
	soFar    int
}

func (p *progressManager) Write(data []byte) (n int, err error) {
	n, err = p.Writer.Write(data)
	if err != nil {
		return
	}

	p.soFar += n

	select {
	case p.Progress <- (float64(p.soFar) / float64(p.Length)):
	default:
		// Couldn't write progress.
	}

	return
}

func CheckForUpdate(currentVersion, platform string) (*Update, updateStatus) {
	updateURL := fmt.Sprintf("http://getmelange.com/api/updates/%s/%s", currentVersion, platform)
	resp, err := http.Get(updateURL)
	if err != nil {
		return nil, updateStatus{
			Msg: "Couldn't get updates, got error",
			err: err,
		}
	}
	defer resp.Body.Close()

	// Check for update code
	if resp.StatusCode == 422 {
		return nil, updateStatus{
			Msg:      "No update available.",
			NoUpdate: true,
		}
	}
	if resp.StatusCode != 200 {
		return nil, updateStatus{
			Msg: fmt.Sprintf("Error getting update feed. %d %s", resp.StatusCode, resp.Body),
			err: nil,
		}
	}

	// Decode the response
	update := &Update{}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(update)
	if err != nil {
		return nil, updateStatus{
			Msg: "Couldn't decode update stream, got error",
			err: err,
		}
	}

	return update, updateStatus{
		Msg:     "Found update",
		Updated: true,
	}
}

// downloadUpdate from site, put it in temporary file
func DownloadUpdate(download string, progress chan<- float64) (string, updateStatus) { // Check the updates site
	// Download the update
	updateData, err := http.Get(download)
	if err != nil {
		return "", updateStatus{
			Msg: "Couldn't download update",
			err: err,
		}
	}
	defer updateData.Body.Close()

	file, err := ioutil.TempFile("", "melange_update")
	if err != nil {
		return "", updateStatus{
			Msg: "Couldn't create temporary file, got error",
			err: err,
		}
	}
	defer file.Close()
	defer os.Remove(file.Name())

	n, err := io.Copy(&progressManager{
		Writer:   file,
		Length:   updateData.ContentLength,
		Progress: progress,
	}, updateData.Body)
	if err != nil {
		return "", updateStatus{
			Msg: "Error downloading data",
			err: err,
		}
	}

	// Unzip the update
	dir, err := ioutil.TempDir("", "melange_update_extract")
	if err != nil {
		return "", updateStatus{
			Msg: "Couldn't get extraction temp dir, got err",
			err: err,
		}
	}

	// Go back to the beginning of the file
	_, err = file.Seek(0, 0)
	if err != nil {
		return "", updateStatus{
			Msg: "Couldn't seek file.",
			err: err,
		}
	}

	// Unzip the update
	err = extractZip(file, n, dir)
	if err != nil {
		return "", updateStatus{
			Msg: "Couldn't unzip file.",
			err: err,
		}
	}

	return dir, updateStatus{
		Msg:     "Updated to directory",
		Updated: true,
	}
}

// install update from temp directory
func InstallUpdate(downloadDir, appDir string) updateStatus {
	var err error
	defer os.Exit(0)

	// Rename old Melange
	oldPath := filepath.Join(appDir, "..", ".melange.old")
	err = os.Rename(appDir, oldPath)
	if err != nil {
		return updateStatus{
			Msg: "Can't rename old melange.",
			err: err,
		}
	}

	// Move in new Melange
	err = os.Rename(downloadDir, appDir)
	if err != nil {
		return updateStatus{
			Msg: "Can't move in new melange",
			err: err,
		}
	}

	// Exec Updater
	err = exec.Command(filepath.Join(appDir, "updater"), "-app="+appDir, "-old="+oldPath).Start()
	if err != nil {
		fmt.Println("Couldn't start updater.")
	}

	return updateStatus{
		Updated: true,
	}
}
