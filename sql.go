package main

import (
	"database/sql"

	_ "github.com/lib/pq"
)

type Storage struct {
	*sql.DB
}

func New(dataSourceName string) (*Storage, error) {
	db, err := sql.Open("postgres", dataSourceName)
	if err != nil {
		return &Storage{}, err
	}
	if err := db.Ping(); err != nil {
		return &Storage{}, err
	}
	return &Storage{DB: db}, nil
}

func (s *Storage) InsertLCGIfNonExist(lcg *LCG, id, last uint64) error {
	_, err := s.Exec("INSERT INTO shorturl.lcg_config(a, b, id,last, uniq) VALUES ($1,$2,$3,$4,true) ON CONFLICT DO NOTHING;", lcg.A, lcg.B, id, last)
	return err
}

func (s *Storage) UpdateLCGID(id uint64) error {
	_, err := s.Exec("UPDATE shorturl.lcg_config SET id=$1 WHERE uniq=true;", id)
	return err
}

func (s *Storage) InsertAccess(access *Access) error {
	_, err := s.Exec("INSERT INTO shorturl.access(date, ip, short_url, ua) VALUES ($1,$2,$3,$4)",
		access.Date, access.IP, access.ShortURL, access.UA)
	return err
}

func (s *Storage) InsertShort(short *Short) error {
	_, err := s.Exec("INSERT INTO shorturl.short(path, raw, text) VALUES ($1,$2,$3)",
		short.Path, short.Raw, short.Text)
	return err
}

func (s *Storage) LCGNext(last uint64) error {
	_, err := s.Exec("UPDATE shorturl.lcg_config SET id=id+1,last=$1;", last)
	return err
}

func (s *Storage) QueryShort(path string) (*Short, error) {
	short := &Short{
		Path: path,
	}
	err := s.QueryRow("SELECT raw, text FROM shorturl.short WHERE path=$1;", path).
		Scan(&short.Raw, &short.Text)
	if err != nil {
		return &Short{}, err
	}
	return short, err
}

func (s *Storage) QueryLCG() (*LCG, uint64, uint64, error) {
	lcg := &LCG{}
	var id, last uint64
	err := s.QueryRow("SELECT a,b,id,last FROM shorturl.lcg_config WHERE uniq=true;").
		Scan(&lcg.A, &lcg.B, &id, &last)
	if err != nil {
		return &LCG{}, 0, 0, err
	}
	return lcg, id, last, err
}

func (s *Storage) QueryLCGIDAndLast() (uint64, uint64, error) {
	var id, last uint64
	err := s.QueryRow("SELECT id,last FROM shorturl.lcg_config WHERE uniq=true;").
		Scan(&id, &last)
	if err != nil {
		return 0, 0, err
	}
	return id, last, err
}

func (s *Storage) PathExist(path string) (bool, error) {
	var exist bool
	err := s.QueryRow("SELECT exists(SELECT path FROM shorturl.short WHERE path=$1);", path).
		Scan(&exist)
	if err != nil {
		return true, err
	}
	return exist, err
}
