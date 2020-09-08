package server

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"zvr/utils"

	"github.com/Sirupsen/logrus"
)

var (
	vyosScriptLock = &sync.Mutex{}
	fileLockPath   = "/home/vyos/zvr/.vyosfilelock"
)

/*
const vyosScriptTempl = `export vyatta_sbindir=/opt/vyatta/sbin
SET=${vyatta_sbindir}/my_set
DELETE=${vyatta_sbindir}/my_delete
COPY=${vyatta_sbindir}/my_copy
MOVE=${vyatta_sbindir}/my_move
RENAME=${vyatta_sbindir}/my_rename
ACTIVATE=${vyatta_sbindir}/my_activate
DEACTIVATE=${vyatta_sbindir}/my_activate
COMMENT=${vyatta_sbindir}/my_comment
COMMIT=${vyatta_sbindir}/my_commit
DISCARD=${vyatta_sbindir}/my_discard
SAVE=${vyatta_sbindir}/vyatta-save-config.pl
API=/bin/cli-shell-api

function atexit() {
    $API teardownSession
}

trap atexit EXIT SIGHUP SIGINT SIGTERM

session_env=$($API getSessionEnv $PPID)
echo $session_env
eval $session_env
$API setupSession

$DISCARD
%s
$COMMIT
if [ $? -ne 0 ]; then
	echo "fail to commit"
	exit 1
fi

`
*/

const vyosScriptTempl = `#!/bin/vbash
source /opt/vyatta/etc/functions/script-template

configure
%s
commit

if [ $? -ne 0 ]; then
	echo "fail to commit"
	exit 1
else
	exit
fi

`

func FindNicNameByMacFromConfiguration(mac, configuration string) (string, bool) {
	parser := NewParserFromConfiguration(configuration)

	c := parser.Tree.Get("interfaces ethernet")
	if c == nil {
		return "", false
	}

	for _, eth := range c.ChildNodeKeys() {
		n := c.Getf("%s hw-id", eth)
		if n == nil {
			continue
		}

		hw := n.Value()
		if strings.ToLower(mac) == hw {
			return eth, true
		}
	}

	return "", false
}

func FindNicNameByMac(mac string) (string, bool) {
	return FindNicNameByMacFromConfiguration(mac, VyosShowConfiguration())
}

func RunVyosScriptAsUserVyos(command string) {
	template := vyosScriptTempl
	command = fmt.Sprintf(template, command)
	tmpfile, err := ioutil.TempFile("", "zvr")
	utils.PanicOnError(err)
	defer os.Remove(tmpfile.Name())

	err = ioutil.WriteFile(tmpfile.Name(), []byte(command), 0777)
	utils.PanicOnError(err)
	tmpfile.Sync()
	tmpfile.Close()
	logrus.Debugf("[Configure VYOS]: %s\n", command)
	bash := utils.Bash{
		Command: fmt.Sprintf(`chown vyos:users %s; chmod +x %s; su - vyos -c %v`, tmpfile.Name(), tmpfile.Name(), tmpfile.Name()),
	}
	bash.Run()
	bash.PanicIfError()
}

func RunVyosScript(command string, args map[string]string) {
	template := vyosScriptTempl
	bash := &utils.Bash{
		Command:   fmt.Sprintf(template, command),
		Arguments: args,
		NoLog:     true,
		UseTemp:   true,
	}
	logrus.Debugf("[Configure VYOS]: %s\n", command)
	bash.Run()
	bash.PanicIfError()
}

func VyosLock(fn CommandHandler) CommandHandler {
	return func(ctx *CommandContext) interface{} {
		vyosScriptLock.Lock()
		defer vyosScriptLock.Unlock()

		if vyosFileLock, err := utils.LockFileExcl(fileLockPath); err == nil {
			defer vyosFileLock.Unlock()
		}

		return fn(ctx)
	}
}

func VyosLockInterface(fn func()) func() {
	return func() {
		vyosScriptLock.Lock()
		defer vyosScriptLock.Unlock()

		if vyosFileLock, err := utils.LockFileExcl(fileLockPath); err == nil {
			defer vyosFileLock.Unlock()
		}

		fn()
	}
}
