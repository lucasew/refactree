package consumer

import "example.com/demo/pkg/githubutil"

func Use() string {
	return githubutil.FuzzToken()
}
