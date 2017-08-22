package system

// test variables
var testNobodyUser = "nobody"
var testGroup = "admin"
var testCat = []string{"/bin/cat", "-"}
var testTrue = []string{"/usr/bin/true"}
var testFalse = []string{"/usr/bin/false"}
var testPrintDir = []string{"/bin/pwd"}
var testSleep = []string{"/bin/sleep", "5"}
var testChildren = []string{"/bin/bash", "-c", "/bin/echo test $(sleep 20)"}
var testGroups = []string{"staff", "admin"}
