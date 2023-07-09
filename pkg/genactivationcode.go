package pkg

import "strings"

func GenActivationCode(installationCode string) string {
	split := strings.Split(strings.ToUpper(installationCode), "")
	return split[10] + "H" + split[11] + "P-" + split[0] + split[2] + split[3] + split[6] + "-" + split[1] + split[4] + split[5] + split[9]
}
