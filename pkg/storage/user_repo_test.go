package storage

import (
	"database/sql"
	"reflect"
	"regexp"
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

	tests := []struct {
		name           string
		args           args
		mockExpectFunc func(m *sqlmock.Sqlmock)
		want           types.AuthenticatedUser
		wantErr        error
		shouldErr      bool
	}{
		{
			"Reading non-existing user should error",
			args{email: "unknown@foo.org"},
			func(m *sqlmock.Sqlmock) {
				mock.ExpectQuery(
					regexp.QuoteMeta("SELECT \"u\".\"id\", \"u\".\"email\", \"u\".\"password\", \"u\".\"created_at\", \"u\".\"updated_at\", \"u\".\"account\" FROM \"auth\".\"users\" AS \"u\""),
				).WillReturnError(sql.ErrNoRows)
			},
			types.AuthenticatedUser{},
			sql.ErrNoRows,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockExpectFunc(&mock)
			got, err := GetAuthenticatedUserByEmail(tt.args.email)
			if (err != nil) != tt.shouldErr {
				t.Errorf("GetAuthenticatedUserByEmail() error = %v, wantErr = %v, shouldErr = %v", err, tt.wantErr, tt.shouldErr)
			}
			if err != tt.wantErr {
				t.Errorf("GetAuthenticatedUserByEmail() error = %v, wantErr = %v", err, tt.wantErr)
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
