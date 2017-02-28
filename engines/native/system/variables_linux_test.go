package system

// test variables
var testGroup = "root"
var testCat = []string{"/bin/cat", "-"}
var testTrue = []string{"/bin/true"}
var testFalse = []string{"/bin/false"}
var testPrintDir = []string{"/bin/pwd"}
var testSleep = []string{"/bin/sleep", "5"}
var testChildren = []string{"/bin/bash", "-c", "/bin/echo test $(sleep 20)"}
var testGroups = []string{}
