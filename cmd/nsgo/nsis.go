package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"text/template"
)

// The Windows installer is NSIS's (https://nsis.sourceforge.io), whose
// compiler, makensis, runs on macOS (brew install makensis), Linux and
// Windows: every build writes the script, <name>.nsi, beside the
// executable, naming the files relative to itself, and compiles it into
// <slug>-<version>-windows-<arch>-setup.exe where makensis is installed. Without
// it the package keeps the script, which compiles anywhere as it is.
//
// The installer installs for the user, with no prompt to elevate, in
// %LOCALAPPDATA%\Programs\<name> by default, and installs again where the
// app is: InstallDirRegKey reads the folder from the uninstall entry, and
// install.ps1 passes no /D, which would replace it. It quits the app
// running from that folder, adds a Start menu shortcut and, from the
// finish page, a desktop one, and an uninstaller that Settings lists,
// which deletes the files the installer put there, never the folder's
// others.

// nsisSetupName is the installer's file name.
func nsisSetupName(p *project, arch string) string {
	return p.releaseName("windows_"+arch, "-setup.exe")
}

// buildNSIS writes the NSIS script for the Windows package in dir, built
// for arch (x64 or arm64), with the icon the installer shows, and compiles
// it when makensis is installed, returning the installer's path.
func buildNSIS(ctx context.Context, p *project, dir, arch string) (string, error) {
	stage(ctx, "writing the installer")
	script, err := nsisScript(p, dir, arch)
	if err != nil {
		return "", err
	}
	for _, icon := range []struct{ name, path string }{
		{script.Icon, p.Windows.InstallerIcon}, {script.UninstallIcon, p.Windows.UninstallerIcon},
	} {
		if icon.name == "" {
			continue
		}
		data, err := p.iconFile(icon.path, ".ico")
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(dir, icon.name), data, 0o644); err != nil {
			return "", err
		}
	}
	var out bytes.Buffer
	if err := nsisTemplate.Execute(&out, script); err != nil {
		return "", err
	}
	nsi := filepath.Join(dir, p.Name+".nsi")
	// NSIS reads a script without a byte order mark in the system's code
	// page, so names that are not ASCII would not match their files.
	if err := os.WriteFile(nsi, append([]byte{0xef, 0xbb, 0xbf}, out.Bytes()...), 0o644); err != nil {
		return "", err
	}
	compiler := nsisCompiler()
	if compiler == "" {
		detailIn(ctx, "install NSIS (brew install makensis) and compile %s for the installer", rel(nsi))
		return "", nil
	}
	stage(ctx, "compiling the installer")
	cmd := exec.CommandContext(ctx, compiler, "-V2", "-INPUTCHARSET", "UTF8", filepath.Base(nsi))
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("makensis failed: %w\n%s", err, out)
	}
	return filepath.Join(dir, script.Output), nil
}

// nsisCompiler finds makensis: on PATH, else where NSIS installs on
// Windows. It returns "" when there is none.
func nsisCompiler() string {
	if path, err := exec.LookPath("makensis"); err == nil {
		return path
	}
	if runtime.GOOS != "windows" {
		return ""
	}
	for _, dir := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles")} {
		path := filepath.Join(dir, "NSIS", "makensis.exe")
		if _, err := os.Stat(path); dir != "" && err == nil {
			return path
		}
	}
	return ""
}

// nsisLanguages are the Modern UI's languages, by the tag of the language
// they are in.
var nsisLanguages = map[string]string{
	"ar": "Arabic", "bg": "Bulgarian", "ca": "Catalan", "cs": "Czech", "da": "Danish",
	"de": "German", "el": "Greek", "es": "Spanish", "fa": "Farsi", "fi": "Finnish",
	"fr": "French", "he": "Hebrew", "hr": "Croatian", "hu": "Hungarian", "id": "Indonesian",
	"it": "Italian", "ja": "Japanese", "ko": "Korean", "lt": "Lithuanian", "nb": "Norwegian",
	"nl": "Dutch", "no": "Norwegian", "pl": "Polish", "pt": "Portuguese", "pt-BR": "PortugueseBR",
	"ro": "Romanian", "ru": "Russian", "sk": "Slovak", "sl": "Slovenian", "sr": "Serbian",
	"sv": "Swedish", "th": "Thai", "tr": "Turkish", "uk": "Ukrainian", "vi": "Vietnamese",
	"zh-Hans": "SimpChinese", "zh-Hant": "TradChinese",
}

// nsisLanguageOf picks the Modern UI's language for a tag: its own, or its
// base language's, with Chinese by script or region.
func nsisLanguageOf(lang string) (string, bool) {
	if name, ok := nsisLanguages[lang]; ok {
		return name, true
	}
	base, rest, _ := strings.Cut(lang, "-")
	if base == "zh" {
		for _, part := range strings.Split(rest, "-") {
			switch part {
			case "Hant", "TW", "HK", "MO":
				return "TradChinese", true
			}
		}
		return "SimpChinese", true
	}
	name, ok := nsisLanguages[base]
	return name, ok
}

// nsisWords are the installer's words that NSIS's language files lack, in
// the languages they are in; others take English's.
var nsisWords = map[string][2]string{
	// The finish page's check box, and what the installer asks when the
	// app is running.
	"English":      {"Create a desktop shortcut", "%s is running. Close it and continue?"},
	"SimpChinese":  {"创建桌面快捷方式", "%s 正在运行。是否关闭它并继续？"},
	"TradChinese":  {"建立桌面捷徑", "%s 正在執行。是否關閉它並繼續？"},
	"Japanese":     {"デスクトップにショートカットを作成", "%s は実行中です。終了して続行しますか？"},
	"Korean":       {"바탕 화면에 바로 가기 만들기", "%s이(가) 실행 중입니다. 종료하고 계속하시겠습니까?"},
	"German":       {"Desktopverknüpfung erstellen", "%s wird ausgeführt. Beenden und fortfahren?"},
	"French":       {"Créer un raccourci sur le bureau", "%s est en cours d’exécution. Le fermer et continuer ?"},
	"Spanish":      {"Crear un acceso directo en el escritorio", "%s se está ejecutando. ¿Cerrarlo y continuar?"},
	"Italian":      {"Crea un collegamento sul desktop", "%s è in esecuzione. Chiuderlo e continuare?"},
	"Portuguese":   {"Criar um atalho no ambiente de trabalho", "%s está em execução. Fechá-lo e continuar?"},
	"PortugueseBR": {"Criar um atalho na área de trabalho", "%s está em execução. Fechá-lo e continuar?"},
	"Russian":      {"Создать ярлык на рабочем столе", "%s запущено. Закрыть его и продолжить?"},
}

// nsisLanguage is a language the installer speaks, and its words.
type nsisLanguage struct {
	Name, Shortcut, Running string
}

// nsisData is what the script says.
type nsisData struct {
	Name, Version, Exe, Output string
	// Icon and UninstallIcon are the setup's and the uninstaller's
	// icons, beside the script.
	Icon, UninstallIcon string
	// UninstallKey is the uninstall entry's key, under HKCU.
	UninstallKey string
	// ARM64 has the installer refuse other systems.
	ARM64     bool
	Languages []nsisLanguage
	// Files and Dirs are what the package holds, relative to it, which
	// the uninstaller removes; Excludes what is beside them but the
	// installer leaves out.
	Files, Dirs, Excludes []string
}

func nsisScript(p *project, dir, arch string) (*nsisData, error) {
	name, err := p.displayName()
	if err != nil {
		return nil, err
	}
	d := &nsisData{
		Name: name, Version: p.Version, Exe: p.Name + ".exe", Output: nsisSetupName(p, arch),
		UninstallKey: `Software\Microsoft\Windows\CurrentVersion\Uninstall\` + p.ID,
		ARM64:        arch == "arm64",
		Excludes:     []string{p.Name + ".nsi", "*-setup.exe"},
	}
	img, err := p.iconImage()
	if err != nil {
		return nil, err
	}
	if img != nil || p.Windows.InstallerIcon != "" {
		d.Icon = p.Name + "-setup.ico"
		d.Excludes = append(d.Excludes, d.Icon)
	}
	if img != nil || p.Windows.UninstallerIcon != "" {
		d.UninstallIcon = p.Name + "-uninstall.ico"
		d.Excludes = append(d.Excludes, d.UninstallIcon)
	}
	langs := []string{"English"}
	if n, err := p.appNames(); err != nil {
		return nil, err
	} else if n != nil {
		for _, lang := range n.languages() {
			if l, ok := nsisLanguageOf(lang); ok && !slices.Contains(langs, l) {
				langs = append(langs, l)
			}
		}
	}
	for _, l := range langs {
		words, ok := nsisWords[l]
		if !ok {
			words = nsisWords["English"]
		}
		d.Languages = append(d.Languages, nsisLanguage{Name: l, Shortcut: words[0], Running: fmt.Sprintf(words[1], name)})
	}
	err = filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil || path == dir {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if !strings.ContainsRune(rel, filepath.Separator) {
			for _, x := range d.Excludes {
				if ok, _ := filepath.Match(x, rel); ok {
					return nil
				}
			}
		}
		rel = strings.ReplaceAll(rel, "/", `\`)
		if e.IsDir() {
			d.Dirs = append(d.Dirs, rel)
		} else {
			d.Files = append(d.Files, rel)
		}
		return nil
	})
	// Deepest first, so each is empty when it is removed.
	slices.Reverse(d.Dirs)
	return d, err
}

// nsisEscape escapes s for an NSIS string, which takes $ for variables.
func nsisEscape(s string) string {
	return strings.NewReplacer(`$`, `$$`, `"`, `$\"`, "\n", `$\n`, "\r", `$\r`, "\t", `$\t`).Replace(s)
}

// nsisString quotes s for NSIS.
func nsisString(s string) string { return `"` + nsisEscape(s) + `"` }

// nsisVersion is a version as Windows' file versions take it, four
// numbers: 1.2.0-beta.1 is 1.2.0.0.
func nsisVersion(v string) string {
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	var out []string
	for _, part := range parts {
		n := 0
		for _, r := range part {
			if r < '0' || r > '9' {
				break
			}
			n = n*10 + int(r-'0')
		}
		out = append(out, fmt.Sprint(n%65536))
		if len(out) == 4 || strings.IndexFunc(part, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			break
		}
	}
	for len(out) < 4 {
		out = append(out, "0")
	}
	return strings.Join(out, ".")
}

// nsisCloseApp is the PowerShell that stops the processes running from the
// folder in NSGO_APP_DIR, or with NSGO_APP_CHECK exits 2 if there are any.
// CIM sees the paths of 64-bit processes from a 32-bit installer too.
const nsisCloseApp = `$$d = $$env:NSGO_APP_DIR.TrimEnd('\') + '\'; ` +
	`$$p = @(Get-CimInstance Win32_Process | Where-Object { $$_.ExecutablePath -and $$_.ExecutablePath.StartsWith($$d, 'OrdinalIgnoreCase') }); ` +
	`if ($$env:NSGO_APP_CHECK) { if ($$p.Count) { exit 2 }; exit 0 }; ` +
	`$$p | ForEach-Object { Stop-Process -Id $$_.ProcessId -Force -ErrorAction SilentlyContinue }; ` +
	`Start-Sleep -Milliseconds 500`

var nsisTemplate = template.Must(template.New("nsi").Funcs(template.FuncMap{
	"q": nsisString,
	// path quotes an NSIS variable, such as $INSTDIR, joined to names.
	"path": func(variable string, names ...string) string {
		return `"` + variable + nsisEscape(strings.Join(names, "")) + `"`
	},
	// cmd is a command line for Exec and ExecWait, its parts joined in an NSIS single-quoted string, in which a quote of the app's name is $\'.
	"cmd": func(parts ...string) string {
		return "'" + strings.ReplaceAll(strings.Join(parts, ""), "'", `$\'`) + "'"
	},
	"quad":  nsisVersion,
	"close": func() string { return nsisCloseApp },
	"amp":   func(s string) string { return strings.ReplaceAll(s, "&", "&&") },
}).Parse(`; Written by nsgo build. Compile with NSIS 3: makensis -INPUTCHARSET UTF8 {{.Exe}}.nsi
Unicode true
ManifestDPIAware true
SetCompressor /SOLID lzma
Name {{q .Name}} {{q (amp .Name)}}
OutFile {{q .Output}}
InstallDir {{path "$LOCALAPPDATA\\Programs\\" .Name}}
; Install again where the app is, the folder the user chose before.
InstallDirRegKey HKCU {{q .UninstallKey}} "InstallLocation"
RequestExecutionLevel user
BrandingText " "
{{- if .Icon}}
!define MUI_ICON {{q .Icon}}
{{- end}}
{{- if .UninstallIcon}}
!define MUI_UNICON {{q .UninstallIcon}}
{{- end}}
!define MUI_FINISHPAGE_RUN {{path "$INSTDIR\\" .Exe}}
!define MUI_FINISHPAGE_SHOWREADME ""
!define MUI_FINISHPAGE_SHOWREADME_NOTCHECKED
!define MUI_FINISHPAGE_SHOWREADME_TEXT "$(NSGO_SHORTCUT)"
!define MUI_FINISHPAGE_SHOWREADME_FUNCTION CreateDesktopShortcut
!include "MUI2.nsh"
!include "x64.nsh"
!include "FileFunc.nsh"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
{{- range .Languages}}
!insertmacro MUI_LANGUAGE "{{.Name}}"
{{- end}}
{{- range .Languages}}
LangString NSGO_SHORTCUT ${LANG_{{.Name}}} {{q .Shortcut}}
LangString NSGO_RUNNING ${LANG_{{.Name}}} {{q .Running}}
{{- end}}
VIProductVersion "{{quad .Version}}"
VIAddVersionKey /LANG=${LANG_ENGLISH} "ProductName" {{q .Name}}
VIAddVersionKey /LANG=${LANG_ENGLISH} "ProductVersion" {{q .Version}}
VIAddVersionKey /LANG=${LANG_ENGLISH} "FileVersion" {{q .Version}}
VIAddVersionKey /LANG=${LANG_ENGLISH} "FileDescription" {{q (print .Name " Setup")}}

Function .onInit
{{- if .ARM64}}
  ${IfNot} ${IsNativeARM64}
    MessageBox MB_OK|MB_ICONSTOP {{q (print .Name " runs on ARM64 Windows alone.")}} /SD IDOK
    Abort
  ${EndIf}
{{- end}}
FunctionEnd

; Quits the app running from $INSTDIR: asks first unless silent.
!macro CloseApp un
Function ${un}CloseApp
  Push $0
  Push $1
  StrCpy $0 "$WINDIR\Sysnative\WindowsPowerShell\v1.0\powershell.exe"
  IfFileExists $0 +2
  StrCpy $0 "$SYSDIR\WindowsPowerShell\v1.0\powershell.exe"
  IfFileExists $0 0 done
  System::Call 'Kernel32::SetEnvironmentVariable(t "NSGO_APP_DIR", t "$INSTDIR")'
  IfSilent close
  System::Call 'Kernel32::SetEnvironmentVariable(t "NSGO_APP_CHECK", t "1")'
  nsExec::Exec ` + "`" + `"$0" -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "{{close}}"` + "`" + `
  Pop $1
  System::Call 'Kernel32::SetEnvironmentVariable(t "NSGO_APP_CHECK", t "")'
  StrCmp $1 "2" 0 done
  MessageBox MB_OKCANCEL|MB_ICONEXCLAMATION "$(NSGO_RUNNING)" IDOK close
  Quit
  close:
  nsExec::Exec ` + "`" + `"$0" -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "{{close}}"` + "`" + `
  Pop $1
  done:
  Pop $1
  Pop $0
FunctionEnd
!macroend
!insertmacro CloseApp ""
!insertmacro CloseApp "un."

Section
  Call CloseApp
  SetOutPath "$INSTDIR"
  File /r{{range .Excludes}} /x {{q .}}{{end}} "*"
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  CreateShortCut {{path "$SMPROGRAMS\\" .Name ".lnk"}} {{path "$INSTDIR\\" .Exe}}
  ; Give desktop toasts a COM activator CLSID on the Start menu shortcut.
  ExecWait {{cmd (path "$INSTDIR\\" .Exe) " --nsgo-register-toast=" (path "$SMPROGRAMS\\" .Name ".lnk")}}
  WriteRegStr HKCU {{q .UninstallKey}} "DisplayName" {{q .Name}}
  WriteRegStr HKCU {{q .UninstallKey}} "DisplayVersion" {{q .Version}}
  WriteRegStr HKCU {{q .UninstallKey}} "DisplayIcon" {{path "$INSTDIR\\" .Exe}}
  WriteRegStr HKCU {{q .UninstallKey}} "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU {{q .UninstallKey}} "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegStr HKCU {{q .UninstallKey}} "QuietUninstallString" '"$INSTDIR\Uninstall.exe" /S'
  WriteRegDWORD HKCU {{q .UninstallKey}} "NoModify" 1
  WriteRegDWORD HKCU {{q .UninstallKey}} "NoRepair" 1
  ; WinSparkle runs the installer with /S /RELAUNCH, once the app quit:
  ; the new version starts again.
  ${GetParameters} $0
  ClearErrors
  ${GetOptions} $0 "/RELAUNCH" $1
  ${IfNot} ${Errors}
    Exec {{cmd (path "$INSTDIR\\" .Exe)}}
  ${EndIf}
SectionEnd

; The finish page offers it, so silent installs make none.
Function CreateDesktopShortcut
  SetOutPath "$INSTDIR"
  CreateShortCut {{path "$DESKTOP\\" .Name ".lnk"}} {{path "$INSTDIR\\" .Exe}}
FunctionEnd

Section "Uninstall"
  Call un.CloseApp
  ; Remove the per-user toast COM registration before deleting the EXE.
  ExecWait {{cmd (path "$INSTDIR\\" .Exe) " --nsgo-unregister-toast"}}
  Delete {{path "$SMPROGRAMS\\" .Name ".lnk"}}
  Delete {{path "$DESKTOP\\" .Name ".lnk"}}
  DeleteRegKey HKCU {{q .UninstallKey}}
  ; What the installer put there alone, never the folder's other files.
{{- range .Files}}
  Delete {{path "$INSTDIR\\" .}}
{{- end}}
{{- range .Dirs}}
  RMDir {{path "$INSTDIR\\" .}}
{{- end}}
  Delete "$INSTDIR\Uninstall.exe"
  RMDir "$INSTDIR"
SectionEnd
`))

// icoFile is an .ico of img in the sizes Windows picks from, each a PNG.
func icoFile(img image.Image) []byte {
	var out bytes.Buffer
	put := func(v any) { binary.Write(&out, binary.LittleEndian, v) }
	put([3]uint16{0, 1, uint16(len(windowsIconSizes))})
	offset := 6 + 16*len(windowsIconSizes)
	var images [][]byte
	for _, size := range windowsIconSizes {
		data := scaledPNG(img, size)
		images = append(images, data)
		side := uint8(size)
		if size >= 256 {
			side = 0
		}
		put([4]uint8{side, side, 0, 0})
		put([2]uint16{1, 32})
		put([2]uint32{uint32(len(data)), uint32(offset)})
		offset += len(data)
	}
	for _, data := range images {
		out.Write(data)
	}
	return out.Bytes()
}
