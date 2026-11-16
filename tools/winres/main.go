// Command winres generates the Windows resource COFF objects for DVPL.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	version := flag.String("version", "dev", "product version")
	flag.Parse()
	if err := generate(*version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(version string) error {
	parts := strings.SplitN(version, ".", 4)
	var nums [4]int
	for i, p := range parts {
		if _, err := fmt.Sscanf(p, "%d", &nums[i]); err != nil {
			nums[i] = 0
		}
		if nums[i] < 0 || nums[i] > 65535 {
			return fmt.Errorf("version component %q outside 0..65535", p)
		}
	}
	// Version strings in the CLI remain verbatim; Windows VERSIONINFO requires
	// a numeric tuple as well, so non-numeric versions map to 0.0.0.0.
	ver := fmt.Sprintf("%d,%d,%d,%d", nums[0], nums[1], nums[2], nums[3])
	quoted := strings.ReplaceAll(version, `"`, `\"`)
	root, err := filepath.Abs(".")
	if err != nil {
		return err
	}
	for _, target := range []struct{ arch, machine string }{{"amd64", "X64"}, {"386", "X86"}} {
		rc := fmt.Sprintf(`1 ICON "%s"
1 VERSIONINFO
 FILEVERSION %s
 PRODUCTVERSION %s
 FILETYPE 1
BEGIN
  BLOCK "StringFileInfo"
  BEGIN
    BLOCK "040904b0"
    BEGIN
      VALUE "ProductName", "DVPL\0"
      VALUE "FileDescription", "Compress and decompress DVPL files\0"
      VALUE "ProductVersion", "%s\0"
      VALUE "FileVersion", "%s\0"
    END
  END
  BLOCK "VarFileInfo"
  BEGIN
    VALUE "Translation", 0x0409, 1200
  END
END
`, filepath.Join(root, "packaging", "windows", "dvpl.ico"), ver, ver, quoted, quoted)
		tmp, err := os.MkdirTemp("", "dvpl-winres-")
		if err != nil {
			return err
		}
		in := filepath.Join(tmp, "dvpl.rc")
		res := filepath.Join(tmp, "dvpl.res")
		if err = os.WriteFile(in, []byte(rc), 0600); err == nil {
			err = run("llvm-rc", "/fo"+res, in)
		}
		out := filepath.Join(root, "cmd", "dvpl", "rsrc_windows_"+target.arch+".syso")
		if err == nil {
			err = run("llvm-cvtres", "/machine:"+target.machine, "/out:"+out, res)
		}
		_ = os.RemoveAll(tmp)
		if err != nil {
			return err
		}
	}
	return nil
}

func run(name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}
