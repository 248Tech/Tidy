//go:build windows

package core

import (
	"os"
	"path/filepath"
	"strings"
)

var blockedProcs = map[string]string{
	"system":             "This is the heart of Windows. Stopping it would crash your PC.",
	"registry":           "Windows needs this to keep its settings. It can't be stopped.",
	"secure system":      "A protected part of Windows security.",
	"memory compression": "Windows uses this to fit more into your memory. It's helping you!",
	"smss.exe":           "Starts your Windows session. Stopping it would crash your PC.",
	"csrss.exe":          "A core part of Windows. Stopping it would crash your PC.",
	"wininit.exe":        "A core part of Windows. Stopping it would crash your PC.",
	"winlogon.exe":       "Handles signing in. Stopping it would sign you out or crash.",
	"services.exe":       "Runs every Windows service. Stopping it would crash your PC.",
	"lsass.exe":          "Keeps your passwords secure. Stopping it forces a restart.",
	"lsaiso.exe":         "Keeps your passwords secure.",
	"fontdrvhost.exe":    "Draws text on screen for Windows.",
}

var cautionProcs = map[string]string{
	"svchost.exe":                 "Runs Windows services. Stopping it may break Wi-Fi, sound or updates until you restart.",
	"dwm.exe":                     "Draws your windows. The screen will flicker while it restarts itself.",
	"explorer.exe":                "Runs your taskbar and desktop. They will disappear until it restarts.",
	"spoolsv.exe":                 "Handles printing.",
	"audiodg.exe":                 "Plays your sound. Audio will stop briefly.",
	"msmpeng.exe":                 "Microsoft Defender antivirus — it protects your PC.",
	"ctfmon.exe":                  "Handles keyboard and typing input.",
	"sihost.exe":                  "Runs parts of the Windows shell.",
	"runtimebroker.exe":           "Manages permissions for Windows apps.",
	"searchhost.exe":              "Powers Windows search.",
	"startmenuexperiencehost.exe": "Runs the Start menu.",
}

func protection(p Process, self uint32) (string, string) {
	if p.PID == self {
		return "blocked", "That's Tidy itself!"
	}
	if p.PID <= 4 {
		return "blocked", blockedProcs["system"]
	}
	n := strings.ToLower(p.Name)
	if r, ok := blockedProcs[n]; ok {
		return "blocked", r
	}
	if r, ok := cautionProcs[n]; ok {
		return "caution", r
	}
	if sysRoot != "" && strings.HasPrefix(strings.ToLower(p.Exe), sysRoot+`\`) {
		return "caution", "This is part of Windows. Stopping it might make something stop working until you restart."
	}
	return "", ""
}

// ProtectedPath reports why a path must not be deleted ("" = OK).
func ProtectedPath(path string) string {
	if !filepath.IsAbs(path) {
		return "Invalid path"
	}
	clean := strings.ToLower(filepath.Clean(path))
	vol := strings.ToLower(filepath.VolumeName(clean))
	if vol == "" || len(clean) <= len(vol)+1 {
		return "You can't delete a whole drive."
	}
	exact := []string{
		sysRoot, vol + `\program files`, vol + `\program files (x86)`, vol + `\programdata`,
		vol + `\users`, strings.ToLower(os.Getenv("USERPROFILE")), vol + `\recovery`, vol + `\boot`,
	}
	for _, e := range exact {
		if e != "" && clean == e {
			return "This is an important system folder."
		}
	}
	if sysRoot != "" && strings.HasPrefix(clean, sysRoot+`\`) && !strings.HasPrefix(clean, sysRoot+`\temp\`) {
		return "Files inside Windows keep your PC running."
	}
	for _, f := range []string{"pagefile.sys", "hiberfil.sys", "swapfile.sys", "dumpstack.log.tmp", "bootmgr"} {
		if clean == vol+`\`+f {
			return "Windows manages this file itself."
		}
	}
	if strings.Contains(clean, `\system volume information`) || strings.HasPrefix(clean, vol+`\$recycle.bin`) {
		return "This is managed by Windows."
	}
	return ""
}
