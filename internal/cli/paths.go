package cli

import (
	"log"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
)

var (
	homeMountRoot  = home("/mnt")
	mountRootsByOS = map[string][]string{
		"darwin": []string{homeMountRoot, "/Volumes"},
		"linux":  []string{homeMountRoot, "/mnt"},
	}
)

func FindDefaultMountRoot() string {
	return firstExistentDir(mountRootsByOS[runtime.GOOS])
}

// firstExistentPath returns the first path that actually exists and is a directory.
// If no directories exist, logs an error message with log.Fatal.
func firstExistentDir(paths []string) string {
	for _, path := range paths {
		dir, err := os.Stat(path)
		if err != nil {
			log.Println("couldn't read", path)
		} else if !dir.IsDir() {
			log.Println("not a directory:", path)
		} else {
			return path
		}
	}

	log.Fatalln("no directories exist:", paths)
	return ""
}

// home returns relPath resolved relative to the current user's home directory.
// If the current user's home directory cannot be found, returns an empty string.
func home(relPath string) string {
	user, err := user.Current()
	if err != nil {
		log.Println("error getting current user:", err)
		return ""
	}
	return filepath.Join(user.HomeDir, relPath)
}
