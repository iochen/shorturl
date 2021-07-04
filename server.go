package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"gopkg.in/yaml.v2"

	"github.com/iochen/shorturl/utils/base64"
)

var cfgFile string

func init() {
	flag.StringVar(&cfgFile, "config", "config.yaml", "Config file")
}

type Config struct {
	Password string `yaml:"password"`
	ID       uint64 `yaml:"id"`
	Last     uint64 `yaml:"last"`
	Postgres string `yaml:"postgres"`
	LCG      *LCG   `yaml:"lcg"`
	COS      *COS   `yaml:"cos"`
}

func main() {
	// check if config file exists
	// or it would create one
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		out, err := yaml.Marshal(&Config{LCG: &LCG{}, COS: &COS{}})
		if err != nil {
			log.Fatalln(err)
		}
		err = ioutil.WriteFile(cfgFile, out, 0644)
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Println("Config file created!")
		return
	}

	// read config file
	bytes, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		log.Fatalln(err)
	}
	// load config
	config := &Config{}
	err = yaml.Unmarshal(bytes, config)
	if err != nil {
		log.Fatalln(err)
	}

	// load postgres
	storage, err := New(config.Postgres)
	if err != nil {
		log.Fatalln(err)
	}
	defer storage.Close()

	// init LCG settings
	err = storage.InsertLCGIfNonExist(config.LCG, config.ID, config.Last)
	if err != nil {
		log.Fatalln(err)
	}
	config.LCG, _, _, err = storage.QueryLCG()
	if err != nil {
		log.Fatalln(err)
	}

	// load MinIO client
	mio, err := config.COS.NewMinIO()
	if err != nil {
		log.Fatalln(err)
	}

	// create a go-fiber app
	app := fiber.New(fiber.Config{
		ErrorHandler: func(ctx *fiber.Ctx, e error) error {
			switch e.(type) {
			case *fiber.Error:
				break
			default:
				log.Println(e)
				e = fiber.NewError(500, "internal server error")
			}

			_ = ctx.SendStatus(e.(*fiber.Error).Code)
			return ctx.JSON(e)
		},
	})

	// serve index
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendFile("views/index.html")
	})
	app.Get("/home", func(c *fiber.Ctx) error {
		return c.SendFile("views/index.html")
	})

	// serve login
	app.Get("/login", func(c *fiber.Ctx) error {
		return c.SendFile("views/login.html")
	})

	// serve short url
	app.Post("/", func(c *fiber.Ctx) error {
		// check token
		if c.Cookies("token") != config.Password {
			c.ClearCookie("token")
			return fiber.NewError(403, "authentication failed")
		}

		// parse request
		short := &Short{}
		if err := c.BodyParser(short); err != nil {
			return err
		}
		// refine request
		short.Path = trim(short.Path)
		short.Raw = strings.TrimSpace(short.Raw)
		// validate
		if len(short.Raw) < 1 {
			return fiber.NewError(400, "invalid raw content: "+short.Raw)
		}

		// random path
		if len(short.Path) < 1 {
		GenLcg:
			// query id_{j} and N_{j-1}
			id, last, err := storage.QueryLCGIDAndLast()
			if err != nil {
				return err
			}
			// calculate id_{j+1} and N_{j}
			current, length := config.LCG.FromLast(id, last)
			if err := storage.LCGNext(current); err != nil {
				return err
			}
			// encode
			short.Path = string(base64.Encode(current, length))
			// check if exists or valid
			exist, err := storage.PathExist(short.Path)
			if err != nil {
				return err
			}
			if exist || !isValidPath(short.Path) {
				goto GenLcg
			}
		} else {
			// custom path
			// check if exists or valid
			exist, err := storage.PathExist(short.Path)
			if err != nil {
				return err
			}
			if exist {
				return fiber.NewError(400, "custom path \""+short.Path+"\" already exist")
			}
			if !isValidPath(short.Path) {
				return fiber.NewError(400, "path \""+short.Path+"\" not allowed")
			}
		}

		// store short url
		err = storage.InsertShort(short)
		if err != nil {
			return err
		}

		return c.JSON(short)
	})

	// serve MinIO sign action
	app.Post("/sign", func(ctx *fiber.Ctx) error {
		// check token
		if ctx.Cookies("token") != config.Password {
			ctx.ClearCookie("token")
			return fiber.NewError(403, "authentication failed")
		}

		// parse request
		req := &struct {
			Filename    string `json:"filename"`
			ContentType string `json:"contentType"`
		}{}
		err := ctx.BodyParser(req)
		if err != nil {
			return err
		}

		// generate random filename (uuid)
		req.Filename = randomFileName(req.Filename)
		// request pre-signed URL
		presignedURL, err := mio.PresignedPutObject(context.Background(), mio.Bucket(), req.Filename, time.Hour*24)
		if err != nil {
			return err
		}

		// load response data
		resp := &struct {
			URL    string `json:"url"`
			Method string `json:"method"`
		}{
			Method: http.MethodPut,
			URL:    presignedURL.String(),
		}

		return ctx.JSON(resp)
	})

	// serve url management
	app.Get("/url", func(ctx *fiber.Ctx) error {
		// check token
		if ctx.Cookies("token") != config.Password {
			ctx.ClearCookie("token")
			return fiber.NewError(403, "authentication failed")
		}
		reader, writer := io.Pipe()
		w := csv.NewWriter(writer)
		go func() {
			rows, err := storage.Query("SELECT path, raw, text FROM shorturl.short;")
			if err != nil {
				log.Println(err)
				return
			}
			for rows.Next() {
				short := &Short{}
				err := rows.Scan(&short.Path, &short.Raw, &short.Text)
				if err != nil {
					log.Println(err)
					return
				}
				text := "0"
				if short.Text {
					text = "1"
				}
				err = w.Write([]string{short.Path, short.Raw, text})
				if err != nil {
					log.Println(err)
					return
				}
			}
			w.Flush()
			writer.Close()
		}()

		ctx.Type("csv", "utf-8")
		ctx.Set("Content-Disposition", "attachment; filename=\"short.csv\"")

		return ctx.SendStream(reader)
	})

	// serve url search
	app.Post("/url/search", func(ctx *fiber.Ctx) error {
		// check token
		if ctx.Cookies("token") != config.Password {
			ctx.ClearCookie("token")
			return fiber.NewError(403, "authentication failed")
		}

		// TODO

		return nil
	})

	// serve objects management
	app.Get("/images", func(ctx *fiber.Ctx) error {
		// check token
		if ctx.Cookies("token") != config.Password {
			ctx.ClearCookie("token")
			return fiber.NewError(403, "authentication failed")
		}
		// TODO
		return nil
	})

	// serve objects search
	app.Post("/images/search", func(ctx *fiber.Ctx) error {
		// check token
		if ctx.Cookies("token") != config.Password {
			ctx.ClearCookie("token")
			return fiber.NewError(403, "authentication failed")
		}
		// TODO
		return nil
	})

	// serve admin
	app.Get("/admin", func(ctx *fiber.Ctx) error {
		// check token
		if ctx.Cookies("token") != config.Password {
			ctx.ClearCookie("token")
			return fiber.NewError(403, "authentication failed")
		}
		reader, writer := io.Pipe()
		w := csv.NewWriter(writer)
		go func() {
			rows, err := storage.Query("SELECT date, ip, short_url, ua FROM shorturl.access;")
			if err != nil {
				log.Println(err)
				return
			}
			for rows.Next() {
				access := &Access{}
				err := rows.Scan(&access.Date, &access.IP, &access.ShortURL, &access.UA)
				if err != nil {
					log.Println(err)
					return
				}
				err = w.Write([]string{access.Date.String(), access.IP, access.ShortURL, access.UA})
				if err != nil {
					log.Println(err)
					return
				}
			}
			w.Flush()
			writer.Close()
		}()

		ctx.Type("csv", "utf-8")
		ctx.Set("Content-Disposition", "attachment; filename=\"access.csv\"")

		return ctx.SendStream(reader)
	})

	// serve url redirect
	app.Get("/:key", func(ctx *fiber.Ctx) error {
		key := ctx.Params("key")
		short, err := storage.QueryShort(key)
		if err != nil {
			if err == sql.ErrNoRows {
				return fiber.NewError(404, "not found")
			}
			return err
		}
		if err := storage.InsertAccess(&Access{
			Date:     time.Now(),
			IP:       ctx.IP(),
			ShortURL: short.Path,
			UA:       ctx.Get("User-Agent"),
		}); err != nil {
			return err
		}
		if short.Text {
			return ctx.SendString(short.Raw)
		}
		return ctx.Redirect(short.Raw, 301)
	})

	// serve site resources
	app.Static("/src", "src")

	// listen and serve
	log.Fatal(app.ListenTLS(":4004",
		"/encrypt/richard/Cert/dev.iochen.com/fullchain.pem",
		"/encrypt/richard/Cert/dev.iochen.com/privkey.pem"))

}

// trim trims spaces and "/" from custom path
func trim(str string) string {
	reg, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		log.Fatal(err)
	}
	return reg.ReplaceAllString(str, "")
}

// isValidPath checks whether short path is valid
func isValidPath(path string) bool {
	// reserved words
	if path == "index" || path == "login" ||
		path == "src" || path == "url" ||
		path == "images" || path == "sign" ||
		path == "image" || path == "admin" {
		return false
	}

	// offensive words
	//if path == "damn" || ... {
	//
	//}

	return true
}

// randomFileName generates a random file name with the same ext ({uuid}.{ext})
// example: foo.bar -> d16da98d-1902-4e52-8b22-8dd792e071a7.bar
func randomFileName(str string) string {
	ext := filepath.Ext(str)
	return uuid.New().String() + ext
}
