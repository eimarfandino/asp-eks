package awsutils

import (
	"encoding/json"
)

type AwsAccount struct {
	ID   string
	Name string
}

type listAccountsOutput struct {
	AccountList []struct {
		AccountId   string `json:"accountId"`
		AccountName string `json:"accountName"`
	} `json:"accountList"`
}

func ParseAccounts(jsonData []byte) []AwsAccount {
	var output listAccountsOutput
	if err := json.Unmarshal(jsonData, &output); err != nil {
		return nil
	}

	var result []AwsAccount
	for _, acc := range output.AccountList {
		result = append(result, AwsAccount{
			ID:   acc.AccountId,
			Name: acc.AccountName,
		})
	}
	return result
}

type listRolesOutput struct {
	RoleList []struct {
		RoleName string `json:"roleName"`
	} `json:"roleList"`
}

func ParseRoles(jsonData []byte) []string {
	var output listRolesOutput
	if err := json.Unmarshal(jsonData, &output); err != nil {
		return nil
	}

	var result []string
	for _, role := range output.RoleList {
		result = append(result, role.RoleName)
	}
	return result
}
