package taskcenter

import (
	"context"
	"errors"
	"strings"
	"time"

	"heyblog-api/internal/platform/apperror"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type RequestConfigInput struct {
	Name       string        `json:"name" minLength:"1" maxLength:"120"`
	Purpose    string        `json:"purpose" enum:"monitor,rss,verification"`
	Enabled    bool          `json:"enabled"`
	Archived   bool          `json:"archived"`
	Revision   int           `json:"revision"`
	Parameters Configuration `json:"parameters"`
}

type RequestConfig struct {
	ID string `json:"id"`
	RequestConfigInput
	References          []string `json:"references"`
	VerificationDefault bool     `json:"verification_default"`
}

func unsignedDenied() error {
	return apperror.New(apperror.KindForbidden, "unsigned_requires_sys_admin", "unsigned requests require system administrator approval")
}

func requestParameters(input RequestConfigInput) (Configuration, error) {
	c := input.Parameters
	if strings.TrimSpace(input.Name) == "" || len(input.Name) > 120 ||
		(input.Purpose != "monitor" && input.Purpose != "rss" && input.Purpose != "verification") {
		return c, invalid(errors.New("invalid request configuration"))
	}
	check := c
	check.Mode, check.IntervalSeconds, check.Timezone = "interval", 3600, "UTC"
	check.Scope, check.SiteIDs, check.Concurrency = "all", []string{}, 1
	if _, err := (ScheduleInput{Name: input.Name, Kind: "monitor", Configuration: check}).Validate(time.Time{}); err != nil {
		return c, invalid(err)
	}
	if c.MaxRedirects < 0 || c.MaxRedirects > 3 || c.TTLSeconds < 60 || c.TTLSeconds > 300 || (c.Carrier != "both" && c.Carrier != "header") {
		return c, invalid(errors.New("invalid redirect or proof settings"))
	}
	return c, nil
}

func (r *Repository) Configurations(ctx context.Context) ([]RequestConfig, error) {
	rows, err := r.db.Query(ctx, `SELECT c.id::text,c.name,c.purpose,c.enabled,c.archived,c.revision,c.parameters,
		ARRAY(SELECT s.id::text FROM operations.schedules s WHERE s.request_config_id=c.id ORDER BY s.id),
		EXISTS(SELECT 1 FROM operations.task_settings t WHERE t.verification_config_id=c.id)
		FROM operations.request_configs c ORDER BY c.created_at,c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []RequestConfig{}
	for rows.Next() {
		var c RequestConfig
		if err := rows.Scan(&c.ID, &c.Name, &c.Purpose, &c.Enabled, &c.Archived, &c.Revision, &c.Parameters, &c.References, &c.VerificationDefault); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (r *Repository) BindVerification(ctx context.Context, actor Actor, id string) error {
	return r.transaction(ctx, func(tx pgx.Tx) error {
		c, err := loadConfiguration(ctx, tx, id, "verification")
		if err != nil {
			return err
		}
		if !c.Signed {
			return unsignedDenied()
		}
		if _, err := tx.Exec(ctx, `UPDATE operations.task_settings SET verification_config_id=$1 WHERE id=1`, id); err != nil {
			return err
		}
		return recordEvent(ctx, tx, taskEvent{Actor: actor.ID, Entity: id, Action: "verification_config_bound"})
	})
}

func (r *Repository) SaveConfiguration(ctx context.Context, actor Actor, id string, input RequestConfigInput) (RequestConfig, error) {
	parameters, err := requestParameters(input)
	if err != nil {
		return RequestConfig{}, err
	}
	if !parameters.Signed && !actor.System {
		return RequestConfig{}, unsignedDenied()
	}
	if !actor.System && (parameters.Carrier != "both" || parameters.TTLSeconds != 300) && id == "" {
		return RequestConfig{}, unsignedDenied()
	}
	result := RequestConfig{ID: id, RequestConfigInput: input, References: []string{}}
	err = r.transaction(ctx, func(tx pgx.Tx) error {
		if id == "" {
			result.ID = uuid.NewString()
			result.Revision = 1
			_, err := tx.Exec(ctx, `INSERT INTO operations.request_configs(id,name,purpose,enabled,archived,parameters)
				VALUES($1,$2,$3,$4,$5,$6)`, result.ID, input.Name, input.Purpose, input.Enabled, input.Archived, parameters)
			if err != nil {
				return err
			}
		} else {
			var old Configuration
			if err := tx.QueryRow(ctx, `SELECT parameters FROM operations.request_configs WHERE id=$1 FOR UPDATE`, id).Scan(&old); err != nil {
				return taskLookup(err)
			}
			if !old.Signed && !actor.System {
				return unsignedDenied()
			}
			if !actor.System && (old.Carrier != parameters.Carrier || old.TTLSeconds != parameters.TTLSeconds || old.Signed != parameters.Signed) {
				return unsignedDenied()
			}
			err := tx.QueryRow(ctx, `UPDATE operations.request_configs SET name=$2,purpose=$3,enabled=$4,archived=$5,
				parameters=$6,revision=revision+1 WHERE id=$1 AND revision=$7 RETURNING revision`,
				id, input.Name, input.Purpose, input.Enabled, input.Archived, parameters, input.Revision).Scan(&result.Revision)
			if err != nil {
				return taskLookup(err)
			}
		}
		return recordEvent(ctx, tx, taskEvent{Actor: actor.ID, Entity: result.ID, Action: "request_config_saved"})
	})
	return result, err
}

func loadConfiguration(ctx context.Context, tx pgx.Tx, id, kind string) (Configuration, error) {
	var parameters Configuration
	err := tx.QueryRow(ctx, `SELECT parameters FROM operations.request_configs
		WHERE id=$1 AND purpose=$2 AND enabled AND NOT archived FOR SHARE`, id, kind).Scan(&parameters)
	return parameters, taskLookup(err)
}

func bindParameters(c, p Configuration) Configuration {
	c.UserAgent, c.Headers = p.UserAgent, p.Headers
	c.TimeoutSeconds, c.IntervalMS, c.Retries = p.TimeoutSeconds, p.IntervalMS, p.Retries
	c.MaxRedirects, c.Signed, c.Carrier, c.TTLSeconds = p.MaxRedirects, p.Signed, p.Carrier, p.TTLSeconds
	return c
}

func taskLookup(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return conflict()
	}
	return err
}
