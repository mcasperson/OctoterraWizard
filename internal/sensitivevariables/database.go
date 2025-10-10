package sensitivevariables

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mcasperson/OctoterraWizard/internal/naming"
	"log"
	"strconv"
	"strings"
	"time"
)
import _ "github.com/microsoft/go-mssqldb"

func GetDatabaseConnection(server string, port string, database string, username string, password string, ctx context.Context) (*sql.DB, error) {
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}

	dsn := "sqlserver://" + username + ":" + password + "@" + server + ":" + fmt.Sprint(portNum) + "?database=" + database
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return nil, err
	}

	db.SetConnMaxLifetime(0)
	db.SetMaxIdleConns(3)
	db.SetMaxOpenConns(3)

	return db, nil
}

func CheckTableExists(ctx context.Context, db *sql.DB, tableName string) (bool, error) {
	timeout, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	rows, err := db.QueryContext(timeout, "SELECT 1 FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = 'dbo' AND TABLE_NAME = '"+tableName+"'")
	if err != nil {
		return false, err
	}

	if !rows.Next() {
		return false, nil
	}

	return true, nil
}

// ExtractVariables extracts sensitive variables from the database and returns them as terraform variable values
func ExtractVariables(server string, port string, database string, username string, password string, masterKey string) (string, error) {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	db, err := GetDatabaseConnection(server, port, database, username, password, ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Println(err.Error())
		}
	}()

	// Test the connection
	if err := PingDatabase(ctx, db); err != nil {
		return "", err
	}

	var sensitiveValues strings.Builder

	// Get the sensitive variables
	sensitiveVars, err := getVariableSetSecrets(ctx, db, masterKey)

	if err != nil {
		return "", err
	}

	sensitiveValues.WriteString(sensitiveVars)

	// Get the account passwords
	accountCreds, err := getAccountCreds(ctx, db, masterKey)

	if err != nil {
		return "", err
	}

	sensitiveValues.WriteString(accountCreds)

	// Get the tenant vars passwords
	tenantVars, err := getTenantVarSensitiveValues(ctx, db, masterKey)

	if err != nil {
		return "", err
	}

	sensitiveValues.WriteString(tenantVars)

	// Get the feed passwords
	feedVars, err := geFeedSensitiveValues(ctx, db, masterKey)

	if err != nil {
		return "", err
	}

	sensitiveValues.WriteString(feedVars)

	// Get the certificates
	certificates, err := getCertificateSensitiveValues(ctx, db, masterKey)

	if err != nil {
		return "", err
	}

	sensitiveValues.WriteString(certificates)

	// Get the git credentials
	gitCreds, err := getGitCredsSensitiveValues(ctx, db, masterKey)

	if err != nil {
		return "", err
	}

	sensitiveValues.WriteString(gitCreds)

	// Get the step template vars
	stepTemplateVars, err := getStepTemplateSensitiveValues(ctx, db, masterKey)

	if err != nil {
		return "", err
	}

	sensitiveValues.WriteString(stepTemplateVars)

	// Get the step vars
	stepVars, err := getStepsSensitiveValues(ctx, db, masterKey)

	if err != nil {
		return "", err
	}

	sensitiveValues.WriteString(stepVars)

	// Get the target vars
	targetVars, err := getTargetSensitiveValues(ctx, db, masterKey)

	if err != nil {
		return "", err
	}

	sensitiveValues.WriteString(targetVars)

	// Get the proxy vars
	proxyVars, err := getMachineProxyPassword(ctx, db, masterKey)

	if err != nil {
		return "", err
	}

	sensitiveValues.WriteString(proxyVars)

	return sensitiveValues.String(), err
}

func PingDatabase(ctx context.Context, db *sql.DB) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return db.PingContext(ctx)
}

func getVariableSetSecrets(ctx context.Context, db *sql.DB, masterKey string) (string, error) {
	var id string
	var jsonValue string
	var isFrozen bool
	var ownerType string

	timeout, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	rows, err := db.QueryContext(timeout, "SELECT Id, JSON, IsFrozen, OwnerType FROM VariableSet")
	if err != nil {
		return "", err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err.Error())
		}
	}()

	var builder strings.Builder

	for rows.Next() {
		if err = rows.Scan(&id, &jsonValue, &isFrozen, &ownerType); err != nil {
			return "", err
		}

		if isFrozen {
			continue
		}

		if ownerType != "Project" && ownerType != "LibraryVariableSet" {
			continue
		}

		var result map[string]interface{}

		if err := json.Unmarshal([]byte(jsonValue), &result); err != nil {
			return "", err
		}

		if variables, ok := result["Variables"].([]interface{}); ok {
			for _, variable := range variables {

				if variableMap, ok := variable.(map[string]interface{}); ok {

					// Don't include the library variable set where we save the secrets
					if fmt.Sprint(variableMap["Name"]) == SecretsVariableName {
						continue
					}

					// only include sensitive variables
					if fmt.Sprint(variableMap["Type"]) != "Sensitive" {
						continue
					}

					variableName := naming.VariableSecretName(fmt.Sprint(variableMap["Id"]))
					variableValue, err := DecryptSensitiveVariable(masterKey, fmt.Sprint(variableMap["Value"]))

					if err != nil {
						return "", err
					}

					if tfVar, err := writeVariableFile(variableName, variableValue); err != nil {
						return "", err
					} else {
						builder.WriteString(tfVar)
					}
				}
			}
		}

	}

	return builder.String(), nil
}

func getAccountCreds(ctx context.Context, db *sql.DB, masterKey string) (string, error) {
	if exists, err := CheckTableExists(ctx, db, "Account"); err == nil {
		if !exists {
			return "", nil
		}
	} else {
		return "", err
	}

	var name string
	var jsonValue string

	timeout, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	rows, err := db.QueryContext(timeout, "SELECT Name, JSON FROM Account")
	if err != nil {
		return "", err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err.Error())
		}
	}()

	var builder strings.Builder

	for rows.Next() {
		if err = rows.Scan(&name, &jsonValue); err != nil {
			return "", err
		}

		var result map[string]interface{}

		if err := json.Unmarshal([]byte(jsonValue), &result); err != nil {
			return "", err
		}

		// Each account type stores different secrets
		password, passwordOk := result["Password"].(string)
		secretKey, secretKeyOk := result["SecretKey"].(string)
		jsonKey, jsonKeyOk := result["JsonKey"].(string)
		privateKeyPassphrase, privateKeyPassphraseOk := result["PrivateKeyPassphrase"].(string)
		privateKeyFile, privateKeyFileOk := result["PrivateKeyFile"].(string)
		token, tokenOk := result["Token"].(string)

		// Must have one sensitive value to extract
		if !(passwordOk || secretKeyOk || jsonKeyOk || privateKeyPassphraseOk || privateKeyFileOk || tokenOk) {
			continue
		}

		variableName := naming.AccountSecretName(fmt.Sprint(result["Name"]))
		variableNameCert := naming.AccountCertName(fmt.Sprint(result["Name"]))

		var variableValue string
		var variableValueCert string

		if passwordOk {
			variableValue, err = DecryptSensitiveVariable(masterKey, fmt.Sprint(password))
		} else if secretKeyOk {
			variableValue, err = DecryptSensitiveVariable(masterKey, fmt.Sprint(secretKey))
		} else if jsonKeyOk {
			variableValue, err = DecryptSensitiveVariable(masterKey, fmt.Sprint(jsonKey))
		} else if privateKeyPassphraseOk && privateKeyFileOk {
			variableValue, err = DecryptSensitiveVariable(masterKey, fmt.Sprint(privateKeyPassphrase))
			variableValueCert, err = DecryptSensitiveVariable(masterKey, fmt.Sprint(privateKeyFile))
		} else if tokenOk {
			variableValue, err = DecryptSensitiveVariable(masterKey, fmt.Sprint(token))
		}

		if err != nil {
			return "", err
		}

		if tfVar, err := writeVariableFile(variableName, variableValue); err != nil {
			return "", err
		} else {
			builder.WriteString(tfVar)
		}

		if tfVar, err := writeVariableFile(variableNameCert, variableValueCert); err != nil {
			return "", err
		} else {
			builder.WriteString(tfVar)
		}

	}

	return builder.String(), nil
}

func getTenantVarSensitiveValues(ctx context.Context, db *sql.DB, masterKey string) (string, error) {
	if exists, err := CheckTableExists(ctx, db, "TenantVariable"); err == nil {
		if !exists {
			return "", nil
		}
	} else {
		return "", err
	}

	var id string
	var jsonValue string

	timeout, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	rows, err := db.QueryContext(timeout, "SELECT Id, JSON FROM TenantVariable")
	if err != nil {
		return "", err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err.Error())
		}
	}()

	var builder strings.Builder

	for rows.Next() {
		if err = rows.Scan(&id, &jsonValue); err != nil {
			return "", err
		}

		var result map[string]interface{}

		if err := json.Unmarshal([]byte(jsonValue), &result); err != nil {
			return "", err
		}

		tentacleVariableName := naming.TenantVariableSecretName(id)

		value, valueOk := result["Value"].(map[string]interface{})

		if !valueOk {
			return "", errors.New("Value is not a map")
		}

		sensitiveValue, sensitiveValueOk := value["SensitiveValue"].(string)

		if sensitiveValueOk {
			decryptedSensitiveValue, err := DecryptSensitiveVariable(masterKey, fmt.Sprint(sensitiveValue))

			if err != nil {
				return "", err
			}

			if tfVar, err := writeVariableFile(tentacleVariableName, decryptedSensitiveValue); err != nil {
				return "", err
			} else {
				builder.WriteString(tfVar)
			}
		}

	}

	return builder.String(), nil
}

func getCertificateSensitiveValues(ctx context.Context, db *sql.DB, masterKey string) (string, error) {
	if exists, err := CheckTableExists(ctx, db, "Certificate"); err == nil {
		if !exists {
			return "", nil
		}
	} else {
		return "", err
	}

	var name string
	var jsonValue string

	timeout, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	rows, err := db.QueryContext(timeout, "SELECT Name, JSON FROM Certificate")
	if err != nil {
		return "", err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err.Error())
		}
	}()

	var builder strings.Builder

	for rows.Next() {
		if err = rows.Scan(&name, &jsonValue); err != nil {
			return "", err
		}

		var result map[string]interface{}

		if err := json.Unmarshal([]byte(jsonValue), &result); err != nil {
			return "", err
		}

		// Each account type stores different secrets
		certificate, certificateOk := result["CertificateData"].(string)

		if !certificateOk {
			return "", errors.New("CertificateData is not a string")
		}

		password, passwordOk := result["Password"].(string)

		certDataName := naming.CertificateDataName(name)
		certPassName := naming.CertificatePasswordName(name)

		certValue, err := DecryptSensitiveVariable(masterKey, fmt.Sprint(certificate))

		if err != nil {
			return "", err
		}

		if tfVar, err := writeVariableFile(certDataName, certValue); err != nil {
			return "", err
		} else {
			builder.WriteString(tfVar)
		}

		if passwordOk {
			passwordValue, err := DecryptSensitiveVariable(masterKey, fmt.Sprint(password))

			if err != nil {
				return "", err
			}

			if tfVar, err := writeVariableFile(certPassName, passwordValue); err != nil {
				return "", err
			} else {
				builder.WriteString(tfVar)
			}
		}

	}

	return builder.String(), nil
}

func geFeedSensitiveValues(ctx context.Context, db *sql.DB, masterKey string) (string, error) {
	if exists, err := CheckTableExists(ctx, db, "Feed"); err == nil {
		if !exists {
			return "", nil
		}
	} else {
		return "", err
	}

	var name string
	var jsonValue string

	timeout, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	rows, err := db.QueryContext(timeout, "SELECT Name, JSON FROM Feed")
	if err != nil {
		return "", err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err.Error())
		}
	}()

	var builder strings.Builder

	for rows.Next() {
		if err = rows.Scan(&name, &jsonValue); err != nil {
			return "", err
		}

		var result map[string]interface{}

		if err := json.Unmarshal([]byte(jsonValue), &result); err != nil {
			return "", err
		}

		// Each account type stores different secrets
		password, passwordOk := result["Password"].(string)
		secretKey, secretKeyOk := result["SecretKey"].(string)

		// Must have one sensitive value to extract
		if !(passwordOk || secretKeyOk) {
			continue
		}

		var variableName string
		var variableValue string
		if passwordOk {
			variableName = naming.FeedSecretName(fmt.Sprint(result["Name"]))
			variableValue, err = DecryptSensitiveVariable(masterKey, fmt.Sprint(password))
		} else if secretKeyOk {
			variableName = naming.FeedSecretKeyName(fmt.Sprint(result["Name"]))
			variableValue, err = DecryptSensitiveVariable(masterKey, fmt.Sprint(secretKey))
		}

		if err != nil {
			return "", err
		}

		if tfVar, err := writeVariableFile(variableName, variableValue); err != nil {
			return "", err
		} else {
			builder.WriteString(tfVar)
		}

	}

	return builder.String(), nil
}

func getGitCredsSensitiveValues(ctx context.Context, db *sql.DB, masterKey string) (string, error) {
	if exists, err := CheckTableExists(ctx, db, "GitCredential"); err == nil {
		if !exists {
			return "", nil
		}
	} else {
		return "", err
	}

	var id string
	var jsonValue string

	timeout, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Not all versions of Octopus have this table
	exists, err := CheckTableExists(ctx, db, "GitCredential")
	if err != nil {
		return "", err
	}

	if !exists {
		return "", nil
	}

	rows, err := db.QueryContext(timeout, "SELECT Id, JSON FROM GitCredential")
	if err != nil {
		return "", err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err.Error())
		}
	}()

	var builder strings.Builder

	for rows.Next() {
		if err = rows.Scan(&id, &jsonValue); err != nil {
			return "", err
		}

		var result map[string]interface{}

		if err := json.Unmarshal([]byte(jsonValue), &result); err != nil {
			return "", err
		}

		gitCredName := naming.GitCredentialSecretName(id)

		details, detailsOk := result["details"].(map[string]interface{})

		if !detailsOk {
			return "", errors.New("details is not a map")
		}

		sensitiveValue, sensitiveValueOk := details["Password"].(string)

		if sensitiveValueOk {
			decryptedSensitiveValue, err := DecryptSensitiveVariable(masterKey, fmt.Sprint(sensitiveValue))

			if err != nil {
				return "", err
			}

			if tfVar, err := writeVariableFile(gitCredName, decryptedSensitiveValue); err != nil {
				return "", err
			} else {
				builder.WriteString(tfVar)
			}
		}

	}

	return builder.String(), nil
}

func getStepTemplateSensitiveValues(ctx context.Context, db *sql.DB, masterKey string) (string, error) {
	if exists, err := CheckTableExists(ctx, db, "ActionTemplate"); err == nil {
		if !exists {
			return "", nil
		}
	} else {
		return "", err
	}

	var jsonValue string

	timeout, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	rows, err := db.QueryContext(timeout, "SELECT JSON FROM ActionTemplate")
	if err != nil {
		return "", err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err.Error())
		}
	}()

	var builder strings.Builder

	for rows.Next() {
		if err = rows.Scan(&jsonValue); err != nil {
			return "", err
		}

		var result map[string]interface{}

		if err := json.Unmarshal([]byte(jsonValue), &result); err != nil {
			return "", err
		}

		// Steps is an array of objects
		parameters, parametersOk := result["Parameters"].([]interface{})

		if !parametersOk {
			continue
		}
		// process each parameter
		for _, parameter := range parameters {
			// convert each parameter to a map
			parametersMap, parametersMapOk := parameter.(map[string]interface{})

			if !parametersMapOk {
				continue
			}

			// get the default value
			defaultValue, defaultValueOk := parametersMap["DefaultValue"]

			if !defaultValueOk {
				continue
			}

			defaultValueMap, defaultValueMapOk := defaultValue.(map[string]interface{})

			if !defaultValueMapOk {
				continue
			}

			// get the sensitive value
			sensitiveValue, sensitiveValueOk := defaultValueMap["SensitiveValue"]

			if !sensitiveValueOk {
				continue
			}

			// At this point we have drilled down into a sensitive value defined in an action property bag
			sensitiveValueValue, sensitiveValueValueOk := sensitiveValue.(string)

			if !sensitiveValueValueOk {
				continue
			}

			// Out of an abundance of caution, make sure the Id property is a string
			templateId, templateIdOk := result["ImmutableId"].(string)

			if !templateIdOk {
				continue
			}

			// Out of an abundance of caution, make sure the parameters has an ID
			parameterId, parameterIdOk := parametersMap["Id"].(string)

			if !parameterIdOk {
				continue
			}

			// We can now decrypt the sensitive value
			variableName := naming.StepTemplateParameterSecretName(templateId, parameterId)
			variableValue, err := DecryptSensitiveVariable(masterKey, sensitiveValueValue)

			if err != nil {
				return "", err
			}

			if tfVar, err := writeVariableFile(variableName, variableValue); err != nil {
				return "", err
			} else {
				builder.WriteString(tfVar)
			}
		}
	}

	return builder.String(), nil
}

func getStepsSensitiveValues(ctx context.Context, db *sql.DB, masterKey string) (string, error) {
	if exists, err := CheckTableExists(ctx, db, "DeploymentProcess"); err == nil {
		if !exists {
			return "", nil
		}
	} else {
		return "", err
	}

	var id string
	var jsonValue string

	timeout, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	rows, err := db.QueryContext(timeout, "SELECT OwnerId, JSON FROM DeploymentProcess")
	if err != nil {
		return "", err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err.Error())
		}
	}()

	var builder strings.Builder

	for rows.Next() {
		if err = rows.Scan(&id, &jsonValue); err != nil {
			return "", err
		}

		var result map[string]interface{}

		if err := json.Unmarshal([]byte(jsonValue), &result); err != nil {
			return "", err
		}

		// Steps is an array of objects
		steps, stepsOk := result["Steps"].([]interface{})

		if !stepsOk {
			continue
		}

		// process each step
		for _, step := range steps {
			// convert each step to a map

			stepMap, stepMapOk := step.(map[string]interface{})

			if !stepMapOk {
				continue
			}

			// get the step actions
			actions, actionsOk := stepMap["Actions"].([]interface{})

			if !actionsOk {
				continue
			}

			// loop over each action
			for _, action := range actions {
				// convert each action to a map
				actionMap, actionMapOk := action.(map[string]interface{})

				if !actionMapOk {
					continue
				}

				// get the properties as a map
				propertiesMap, propertiesMapOk := actionMap["Properties"].(map[string]interface{})

				if !propertiesMapOk {
					continue
				}

				// loop over each key value pair
				for propertyName, propertyValue := range propertiesMap {
					// Get properties that are map. These are secrets. Regular properties are just strings.
					propertyValueMap, propertyValueMapOk := propertyValue.(map[string]interface{})

					if !propertyValueMapOk {
						continue
					}

					// get the sensitive value
					sensitiveValue, sensitiveValueOk := propertyValueMap["SensitiveValue"]

					if !sensitiveValueOk {
						continue
					}

					// At this point we have drilled down into a sensitive value defined in an action property bag
					sensitiveValueValue, sensitiveValueValueOk := sensitiveValue.(string)

					if !sensitiveValueValueOk {
						continue
					}

					// Out of an abundance of caution, make sure the Id property is a string
					actionId, actionIdOk := actionMap["Id"].(string)

					if !actionIdOk {
						continue
					}

					// We can now decrypt the sensitive value
					variableName := naming.StepPropertySecretName(id, actionId, propertyName)
					variableValue, err := DecryptSensitiveVariable(masterKey, sensitiveValueValue)

					if err != nil {
						return "", err
					}

					if tfVar, err := writeVariableFile(variableName, variableValue); err != nil {
						return "", err
					} else {
						builder.WriteString(tfVar)
					}
				}
			}
		}
	}

	return builder.String(), nil
}

func getTargetSensitiveValues(ctx context.Context, db *sql.DB, masterKey string) (string, error) {
	if exists, err := CheckTableExists(ctx, db, "Machine"); err == nil {
		if !exists {
			return "", nil
		}
	} else {
		return "", err
	}

	var name string
	var jsonValue string

	timeout, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	rows, err := db.QueryContext(timeout, "SELECT Name, JSON FROM Machine")
	if err != nil {
		return "", err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err.Error())
		}
	}()

	var builder strings.Builder

	for rows.Next() {
		if err = rows.Scan(&name, &jsonValue); err != nil {
			return "", err
		}

		var result map[string]interface{}

		if err := json.Unmarshal([]byte(jsonValue), &result); err != nil {
			return "", err
		}

		// Note that as at 0.40.4 the TF provider does not expose the password for an offline target
		gitCredName := naming.MachineSecretName(name)

		endpoint, endpointOk := result["Endpoint"].(map[string]interface{})

		if !endpointOk {
			return "", errors.New("Endpoint is not a map")
		}

		sensitiveValue, sensitiveValueOk := endpoint["SensitiveVariablesEncryptionPassword"].(string)

		if sensitiveValueOk {
			decryptedSensitiveValue, err := DecryptSensitiveVariable(masterKey, fmt.Sprint(sensitiveValue))

			if err != nil {
				return "", err
			}

			if tfVar, err := writeVariableFile(gitCredName, decryptedSensitiveValue); err != nil {
				return "", err
			} else {
				builder.WriteString(tfVar)
			}
		}

	}

	return builder.String(), nil
}

func getMachineProxyPassword(ctx context.Context, db *sql.DB, masterKey string) (string, error) {
	if exists, err := CheckTableExists(ctx, db, "Proxy"); err == nil {
		if !exists {
			return "", nil
		}
	} else {
		return "", err
	}

	var name string
	var jsonValue string

	timeout, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	rows, err := db.QueryContext(timeout, "SELECT Name, JSON FROM Proxy")
	if err != nil {
		return "", err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Println(err.Error())
		}
	}()

	var builder strings.Builder

	for rows.Next() {
		if err = rows.Scan(&name, &jsonValue); err != nil {
			return "", err
		}

		var result map[string]interface{}

		if err := json.Unmarshal([]byte(jsonValue), &result); err != nil {
			return "", err
		}

		// Each account type stores different secrets
		password, passwordOk := result["Password"].(string)

		// Must have one sensitive value to extract
		if !(passwordOk) {
			continue
		}

		variableName := naming.MachineProxyPassword(fmt.Sprint(name))
		variableValue, err := DecryptSensitiveVariable(masterKey, fmt.Sprint(password))

		if err != nil {
			return "", err
		}

		if tfVar, err := writeVariableFile(variableName, variableValue); err != nil {
			return "", err
		} else {
			builder.WriteString(tfVar)
		}

	}

	return builder.String(), nil
}

func writeVariableFile(variableName string, variableValue string) (string, error) {
	if variableValue == "" {
		return "", nil
	}

	escapedValue, err := json.Marshal(variableValue)

	if err != nil {
		return "", err
	}

	return variableName + " = " + string(escapedValue) + "\n", nil
}
