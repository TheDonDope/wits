package storage

import (
	"database/sql"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/TheDonDope/wits/pkg/types"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func TestGetAuthenticatedUserByEmail(t *testing.T) {
	type args struct {
		email string
	}

	// Create a new mock database connection
	db, mock, err := sqlmock.New()

	if err != nil {
		t.Fatalf("failed to create mock db: %v", err)
	}
	defer db.Close()

	// Set up the BunDB to use the mock database
	BunDB = bun.NewDB(db, pgdialect.New())

	unknownEmail := "unknown@foo.org"
	unknownUser := types.AuthenticatedUser{
		Email: unknownEmail,
	}

	tests := []struct {
		name      string
		args      args
		want      types.AuthenticatedUser
		wantErr   error
		shouldErr bool
	}{
		{"Unknown user ID should error", args{email: unknownEmail}, unknownUser, sql.ErrNoRows, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Expect a select query and mock the result
			mock.ExpectQuery("SELECT (.+) FROM auth.users").WithArgs(unknownEmail).WillReturnError(sql.ErrNoRows)
			got, err := GetAuthenticatedUserByEmail(tt.args.email)
			if (err != nil) != tt.shouldErr {
				t.Errorf("GetAuthenticatedUserByEmail() error = %v, wantErr = %v, shouldErr = %v", err, tt.wantErr, tt.shouldErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAuthenticatedUserByEmail() = %v, want %v", got, tt.want)
			}
			// Ensure all expectations were met
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unfulfilled expectations: %s", err)
			}
		})
	}
}
