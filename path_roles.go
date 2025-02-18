package cognito

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/errwrap"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	rolesStoragePath = "roles"

	credentialTypeSP = 0
)

// roleEntry is a Vault role construct that maps to cognito configuration
type roleEntry struct {
	CognitoPoolUrl  string `json:"cognito_pool_url"`
	AppClientSecret string `json:"app_client_secret"`
}

func pathsRole(b *cognitoSecretBackend) []*framework.Path {
	return []*framework.Path{
		{
			Pattern: "roles/" + framework.GenericNameRegex("name"),
			Fields: map[string]*framework.FieldSchema{
				"name": {
					Type:        framework.TypeLowerCaseString,
					Description: "Name of the role.",
				},
				"cognito_pool_url": {
					Type:        framework.TypeString,
					Description: "cognito_pool_url",
				},
				"app_client_secret": {
					Type:        framework.TypeString,
					Description: "app_client_secret.",
				},
			},
			Callbacks: map[logical.Operation]framework.OperationFunc{
				logical.ReadOperation:   b.pathRoleRead,
				logical.CreateOperation: b.pathRoleUpdate,
				logical.UpdateOperation: b.pathRoleUpdate,
				logical.DeleteOperation: b.pathRoleDelete,
			},
			HelpSynopsis:    roleHelpSyn,
			HelpDescription: roleHelpDesc,
			ExistenceCheck:  b.pathRoleExistenceCheck,
		},
		{
			Pattern: "roles/?",
			Callbacks: map[logical.Operation]framework.OperationFunc{
				logical.ListOperation: b.pathRoleList,
			},
			HelpSynopsis:    roleListHelpSyn,
			HelpDescription: roleListHelpDesc,
		},
	}

}

// pathRoleUpdate creates or updates Vault roles.
//
// Basic validity check are made to verify that the provided fields meet requirements
// for the given credential type.
//
func (b *cognitoSecretBackend) pathRoleUpdate(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	var resp *logical.Response

	// load or create role
	name := d.Get("name").(string)
	role, err := getRole(ctx, name, req.Storage)
	if err != nil {
		return nil, errwrap.Wrapf("error reading role: {{err}}", err)
	}

	if role == nil {
		if req.Operation == logical.UpdateOperation {
			return nil, errors.New("role entry not found during update operation")
		}
		role = &roleEntry{}
	}

	// update and verify Application Object ID if provided
	if cognitoPoolUrl, ok := d.GetOk("cognito_pool_url"); ok {
		role.CognitoPoolUrl = cognitoPoolUrl.(string)
	}

	if appClientSecret, ok := d.GetOk("app_client_secret"); ok {
		role.AppClientSecret = appClientSecret.(string)
	}

	// save role
	err = saveRole(ctx, req.Storage, role, name)
	if err != nil {
		return nil, errwrap.Wrapf("error storing role: {{err}}", err)
	}

	return resp, nil
}

func (b *cognitoSecretBackend) pathRoleRead(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	var data = make(map[string]interface{})

	name := d.Get("name").(string)

	r, err := getRole(ctx, name, req.Storage)
	if err != nil {
		return nil, errwrap.Wrapf("error reading role: {{err}}", err)
	}

	if r == nil {
		return nil, nil
	}

	data["cognito_pool_url"] = r.CognitoPoolUrl
	data["app_client_secret"] = r.AppClientSecret

	return &logical.Response{
		Data: data,
	}, nil
}

func (b *cognitoSecretBackend) pathRoleList(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	roles, err := req.Storage.List(ctx, rolesStoragePath+"/")
	if err != nil {
		return nil, errwrap.Wrapf("error listing roles: {{err}}", err)
	}

	return logical.ListResponse(roles), nil
}

func (b *cognitoSecretBackend) pathRoleDelete(ctx context.Context, req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	name := d.Get("name").(string)

	err := req.Storage.Delete(ctx, fmt.Sprintf("%s/%s", rolesStoragePath, name))
	if err != nil {
		return nil, errwrap.Wrapf("error deleting role: {{err}}", err)
	}

	return nil, nil
}

func (b *cognitoSecretBackend) pathRoleExistenceCheck(ctx context.Context, req *logical.Request, d *framework.FieldData) (bool, error) {
	name := d.Get("name").(string)

	role, err := getRole(ctx, name, req.Storage)
	if err != nil {
		return false, errwrap.Wrapf("error reading role: {{err}}", err)
	}

	return role != nil, nil
}

func saveRole(ctx context.Context, s logical.Storage, c *roleEntry, name string) error {
	entry, err := logical.StorageEntryJSON(fmt.Sprintf("%s/%s", rolesStoragePath, name), c)
	if err != nil {
		return err
	}

	return s.Put(ctx, entry)
}

func getRole(ctx context.Context, name string, s logical.Storage) (*roleEntry, error) {
	entry, err := s.Get(ctx, fmt.Sprintf("%s/%s", rolesStoragePath, name))
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, nil
	}

	role := new(roleEntry)
	if err := entry.DecodeJSON(role); err != nil {
		return nil, err
	}
	return role, nil
}

const roleHelpSyn = "Manage the Vault roles used to generate cognito credentials."
const roleHelpDesc = `
This path allows you to read and write roles that are used to generate cognito login
credentials.

If the backend is mounted at "cognito", you would create a Vault role at "cognito/roles/my_role",
and request credentials from "cognito/creds/my_role".
`
const roleListHelpSyn = `List existing roles.`
const roleListHelpDesc = `List existing roles by name.`
