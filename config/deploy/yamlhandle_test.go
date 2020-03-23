/**
 * 功能描述: yaml文件处理,去除description字段
 * @Date: 2019-12-03
 * @author: lixiaoming
 */
package deploy

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func TestHandle(t *testing.T) {
	fi, err := os.Open("/Users/lixiaoming/Documents/git/dashuo/crd/application/config/deploy/deploy-all.yaml")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	newFile, err := os.Create("/Users/lixiaoming/Documents/git/dashuo/crd/application/config/deploy/deploy-all-filter.yaml")
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	defer fi.Close()
	br := bufio.NewReader(fi)
	bankNum := 0
	isDescription := false
	lineNumber := 1
	for {
		a, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		line := string(a)
		if isDescription {
			pristr := ""
			for i := 0; i < bankNum+2; i++ {
				pristr = pristr + " "
			}
			if strings.HasPrefix(line, pristr) {
				isDescription = true
			} else {
				isDescription = false
				newFile.WriteString(fmt.Sprintf("%s\n", line))
				fmt.Println(line)
			}

		} else {
			if strings.Contains(line, "description:") {
				bankNum = strings.Index(line, "description:")
				if strings.HasSuffix(line, "description:") {
					newFile.WriteString(fmt.Sprintf("%s\n", line))
					fmt.Println(line)
					isDescription = false
				} else {
					isDescription = true
				}
			} else {

				//if strings.Contains(line, " - ") {
				//	line = strings.Replace(line, "  -", "-", -1)
				//}
				fmt.Println(line)
				writerStr := fmt.Sprintf("%s\n", line)
				newFile.WriteString(writerStr)
				isDescription = false
			}
		}
		lineNumber++
	}
	newFile.Close()
}
