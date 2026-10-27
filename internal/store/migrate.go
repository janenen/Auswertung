package store

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// migrate upgrades databases written by older versions of the Python
// implementation. It works on the raw JSON so it can read old layouts.
func migrate(dir string) error {
	if err := migrateMatches(dir); err != nil {
		return err
	}
	if err := migrateUsers(dir); err != nil {
		return err
	}
	return migrateIni(dir)
}

type rawDB = map[string]any

func readRaw(path string) (rawDB, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var db rawDB
	if err := dec.Decode(&db); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return db, nil
}

func writeRaw(path string, db rawDB) error {
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data)
}

func version(db rawDB) int {
	switch v := db["version"].(type) {
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	case int:
		return v
	}
	return 0
}

func object(v any) map[string]any {
	m, _ := v.(map[string]any)
	if m == nil {
		return map[string]any{}
	}
	return m
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func migrateMatches(dir string) error {
	path := filepath.Join(dir, "matches.json")
	matches, err := readRaw(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if version(matches) == matchDBVersion {
		return nil
	}
	if version(matches) == 0 {
		// v0 stored Teiler in 1/10 and coordinates in 1/100 as integers.
		for _, m := range object(matches["matches"]) {
			shots, _ := object(m)["shots"].([]any)
			for _, s := range shots {
				shot := object(s)
				teiler, ok := shot["teiler"].(json.Number)
				if !ok || strings.ContainsAny(string(teiler), ".eE") {
					continue
				}
				for key, div := range map[string]float64{"teiler": 10, "x": 100, "y": 100} {
					if n, ok := shot[key].(json.Number); ok {
						f, _ := n.Float64()
						shot[key] = f / div
					}
				}
			}
		}
		matches["version"] = 1
	}
	if version(matches) == 1 {
		// v1 embedded the shooter; v2 references users by ID.
		usersPath := filepath.Join(dir, "users.json")
		users, err := readRaw(usersPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if users == nil {
			users = rawDB{"users": map[string]any{}}
		}
		userMap := object(users["users"])
		users["users"] = userMap
		for _, m := range object(matches["matches"]) {
			match := object(m)
			shooter := object(match["shooter"])
			name := str(shooter["name"])
			id := ""
			for uid, u := range userMap {
				user := object(u)
				if str(object(user["shooter"])["name"]) == name || str(user["name"]) == name {
					id = uid
					break
				}
			}
			if id == "" {
				id = uuid.NewString()
				userMap[id] = map[string]any{"name": name, "birthday": str(shooter["birthday"]), "id": id}
			}
			match["shooter"] = id
		}
		matches["version"] = 2
		if err := writeRaw(usersPath, users); err != nil {
			return err
		}
	}
	log.Printf("migrated matches.json to version %d", matchDBVersion)
	return writeRaw(path, matches)
}

func migrateUsers(dir string) error {
	path := filepath.Join(dir, "users.json")
	users, err := readRaw(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if version(users) == userDBVersion {
		return nil
	}
	// v0 wrapped name and birthday in a "shooter" object.
	converted := map[string]any{}
	for key, u := range object(users["users"]) {
		user := object(u)
		if shooter, ok := user["shooter"]; ok {
			user["name"] = str(object(shooter)["name"])
			user["birthday"] = str(object(shooter)["birthday"])
			delete(user, "shooter")
		}
		if str(user["id"]) == "" {
			user["id"] = key
		}
		for _, list := range []string{"matches", "competitions"} {
			if _, ok := user[list].([]any); !ok {
				user[list] = []any{}
			}
		}
		if _, ok := user["niceness"]; !ok {
			user["niceness"] = 0
		}
		converted[str(user["id"])] = user
	}
	log.Printf("migrated users.json to version %d", userDBVersion)
	return writeRaw(path, rawDB{"users": converted, "version": userDBVersion})
}

// migrateIni imports users from the legacy schuetzen.ini next to the db
// directory when no users.json exists yet.
func migrateIni(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, "users.json")); err == nil {
		return nil
	}
	iniPath := filepath.Join(filepath.Dir(filepath.Clean(dir)), "schuetzen.ini")
	f, err := os.Open(iniPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	users := map[string]any{}
	section := ""
	name, niceness := "", 0
	flush := func() {
		if section != "" && section != "Neu" && section != "NeuerSchütze" && name != "" {
			id := uuid.NewString()
			users[id] = map[string]any{
				"name": name, "birthday": "", "niceness": niceness, "id": id,
				"matches": []any{}, "competitions": []any{},
			}
		}
		name, niceness = "", 0
	}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			flush()
			section = line[1 : len(line)-1]
		default:
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				key, value, _ = strings.Cut(line, ":")
			}
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "name":
				name = strings.TrimSpace(value)
			case "niceness":
				niceness, _ = strconv.Atoi(strings.TrimSpace(value))
			}
		}
	}
	flush()
	f.Close()
	if err := scanner.Err(); err != nil {
		return err
	}
	if err := writeRaw(filepath.Join(dir, "users.json"), rawDB{"users": users, "version": userDBVersion}); err != nil {
		return err
	}
	log.Printf("imported %d users from %s", len(users), iniPath)
	return os.Remove(iniPath)
}
