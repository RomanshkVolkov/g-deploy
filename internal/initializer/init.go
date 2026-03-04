package initializer

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/RomanshkVolkov/g-deploy/internal/assets"
	"github.com/RomanshkVolkov/g-deploy/internal/detector"
)

// Run detects the framework in the current working directory and copies
// the appropriate Dockerfile and CI/deployment files into it.
func Run() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	logf("[*] Detecting framework in %s", cwd)

	fw := detector.Detect(cwd)
	if fw == nil {
		return fmt.Errorf("could not detect framework — ensure you are in a project directory")
	}

	logf("[*] Detected: %s (port: %d, dockerfile: %s)", fw.Name, fw.Port, fw.Dockerfile)

	// Copy .deploy/ and .github/ directories
	for _, dir := range []string{".deploy", ".github"} {
		dst := filepath.Join(cwd, dir)
		if err := copyDir(assets.FS, dir, dst); err != nil {
			return fmt.Errorf("failed to copy %s/: %w", dir, err)
		}
		logf("[*] Copied %s/", dir)
	}

	// Copy .dockerignore
	if err := copyAsset(".dockerignore", filepath.Join(cwd, ".dockerignore")); err != nil {
		return fmt.Errorf("failed to copy .dockerignore: %w", err)
	}
	logf("[*] Copied .dockerignore")

	// Copy the detected Dockerfile as Dockerfile
	if err := copyAsset(fw.Dockerfile, filepath.Join(cwd, "Dockerfile")); err != nil {
		return fmt.Errorf("failed to copy %s as Dockerfile: %w", fw.Dockerfile, err)
	}
	logf("[*] Copied %s → Dockerfile", fw.Dockerfile)

	// Ensure shell scripts are executable
	shellScripts, _ := filepath.Glob(filepath.Join(cwd, ".deploy", "*.sh"))
	for _, script := range shellScripts {
		if err := os.Chmod(script, 0755); err != nil {
			logf("[!] Warning: could not chmod %s: %v", script, err)
		}
	}

	logf("[*] Done! Review .deploy/ and .github/workflows/dev.yml for project-specific settings.")
	return nil
}

func copyDir(fsys fs.FS, srcDir, dstDir string) error {
	return fs.WalkDir(fsys, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(srcDir, path)
		dst := filepath.Join(dstDir, rel)

		if d.IsDir() {
			return os.MkdirAll(dst, 0755)
		}
		return copyFromFS(fsys, path, dst)
	})
}

func copyAsset(assetPath, dst string) error {
	return copyFromFS(assets.FS, assetPath, dst)
}

func copyFromFS(fsys fs.FS, src, dst string) error {
	in, err := fsys.Open(src)
	if err != nil {
		return fmt.Errorf("open asset %s: %w", src, err)
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func logf(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}
