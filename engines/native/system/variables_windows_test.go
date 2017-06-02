package system

// test variables
var testGroup = "Administrator"
var testCat = []string{`c:\Windows\system32\cmd.exe`, "/C", "type con"}
var testTrue = []string{`c:\Windows\system32\cmd.exe`, "/C", "exit 0"}
var testFalse = []string{`c:\Windows\system32\cmd.exe`, "/C", "exit 1"}
var testPrintDir = []string{`c:\Windows\system32\cmd.exe`, "/C", "cd"}
var testSleep = []string{`c:\Windows\system32\notepad.exe`}
var testChildren = []string{`c:\Windows\system32\cmd.exe`, "/C", `c:\Windows\system32\notepad.exe`}
var testGroups = []string{}
