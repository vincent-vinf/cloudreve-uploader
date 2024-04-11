package cloudreve

import (
	"cloudreve_uploader/pkg/config"
	"cloudreve_uploader/pkg/utils"
	"context"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/gabriel-vasile/mimetype"
	"github.com/go-resty/resty/v2"
	"github.com/goccy/go-json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
)

var (
	logger = utils.GetLogger()
)

type Client interface {
	Login() error
	Upload(files []string, remotePath string) error
	DirectLinks(files []string, remotePath string) ([]string, error)
}

type ClientImpl struct {
	config config.Config
	ctx    context.Context

	url      *url.URL
	client   *resty.Client
	policyID string
}

func NewClient(ctx context.Context, config config.Config) (Client, error) {
	u, err := url.Parse(config.Server)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server url: %w", err)
	}
	u.JoinPath()
	return &ClientImpl{
		config: config,
		ctx:    ctx,
		url:    u,
		client: resty.New(),
	}, nil
}

func (c *ClientImpl) Login() error {
	resp, err := c.client.R().
		SetContext(c.ctx).
		SetBody(map[string]any{"userName": c.config.Username, "Password": c.config.Password}).
		Post(c.url.JoinPath("api/v3/user/session").String())
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("login failed: %s, %s", resp.Status(), string(resp.Body()))
	}
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		return fmt.Errorf("login failed: empty cookies")
	}
	c.client.SetCookie(cookies[0])
	logger.Infof("login success to %s as %s", c.config.Server, c.config.Username)

	return nil
}

func (c *ClientImpl) Upload(files []string, remotePath string) error {
	for _, file := range files {
		err := c.upload(file, remotePath)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *ClientImpl) DirectLinks(files []string, remotePath string) ([]string, error) {
	d, err := c.listDirectory(remotePath)
	if err != nil {
		return nil, fmt.Errorf("list dir failed: %w", err)
	}
	filenames := make([]string, len(files))
	for i, f := range files {
		filenames[i] = path.Base(f)
	}
	ids := make([]string, len(filenames))
	for i, file := range filenames {
		for j := len(d.Data.Objects) - 1; j >= 0; j-- {
			if d.Data.Objects[j].Name == file {
				ids[i] = d.Data.Objects[j].ID
				break
			}
		}
		if ids[i] == "" {
			return nil, fmt.Errorf("not fount %s", path.Join(remotePath, file))
		}
	}
	links, err := c.directLinksByID(ids, filenames)
	if err != nil {
		return nil, err
	}

	return links, nil
}

func (c *ClientImpl) directLinksByID(ids, files []string) ([]string, error) {
	resp, err := c.client.R().
		SetContext(c.ctx).
		SetBody(map[string]any{"items": ids}).
		Post(c.url.JoinPath("api/v3/file/source").String())
	if err != nil {
		return nil, fmt.Errorf("get direct link failed: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("get direct link failed: %s, %s", resp.Status(), string(resp.Body()))
	}
	s := source{}
	err = json.Unmarshal(resp.Body(), &s)
	if err != nil {
		return nil, fmt.Errorf("unmarshal direct link failed: %w", err)
	}
	if s.Code != 0 {
		return nil, fmt.Errorf("get direct link failed: %d, %s", s.Code, s.Msg)
	}

	m := make(map[string]string, len(s.Data))
	for _, d := range s.Data {
		m[d.Name] = d.URL
	}

	links := make([]string, len(ids))
	for i, file := range files {
		var ok bool
		links[i], ok = m[file]
		if !ok {
			return nil, fmt.Errorf("not found direct link %s", file)
		}
	}

	return links, nil
}

type source struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"data"`
}

func (c *ClientImpl) upload(file string, remotePath string) error {
	stat, err := os.Stat(file)
	if err != nil {
		return fmt.Errorf("read file %s failed: %w", file, err)
	}
	if stat.IsDir() {
		return fmt.Errorf("%s is a directory, not support", file)
	}
	if remotePath == "" {
		return fmt.Errorf("remote path is empty")
	}
	logger.Infof("upload file %s to %s", file, remotePath)

	policy, err := c.getPolicyID()
	if err != nil {
		return fmt.Errorf("get policy id failed: %w", err)
	}
	logger.Infof("user %s has policy: %s", c.config.Username, policy)
	// create upload session
	resp, err := c.client.R().
		SetContext(c.ctx).
		SetBody(map[string]any{
			"last_modified": stat.ModTime().Unix(),
			"mime_type":     miteType(file),
			"name":          stat.Name(),
			"path":          remotePath,
			"policy_id":     policy,
			"size":          stat.Size(),
		}).
		Put(c.url.JoinPath("api/v3/file/upload").String())
	if err != nil {
		return err
	}
	u := upload{}
	err = json.Unmarshal(resp.Body(), &u)
	if err != nil {
		return fmt.Errorf("unmarshal upload response failed: %w", err)
	}
	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("open file %s failed: %w", file, err)
	}
	defer f.Close()
	if u.Code != 0 {
		return fmt.Errorf("upload %s failed(%d): %s", file, u.Code, u.Msg)
	}
	buff := make([]byte, u.Data.ChunkSize)
	chuckNumber := int(stat.Size()/u.Data.ChunkSize) + 1
	logger.Infof("file %s size: %s, chuck size %s, chuck number: %d",
		file,
		humanize.Bytes(uint64(stat.Size())),
		humanize.Bytes(uint64(u.Data.ChunkSize)),
		chuckNumber,
	)
	for i := 0; i < chuckNumber; i++ {
		logger.Infof("uploading file chuck %d", i)
		err = c.uploadChunk(f, i, buff, u.Data.SessionID)
		if err != nil {
			return err
		}
	}

	return nil
}

type upload struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		ChunkSize int64  `json:"chunkSize"`
		Expires   int64  `json:"expires"`
		SessionID string `json:"sessionID"`
	} `json:"data"`
}

type directory struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Objects []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"objects"`
		Policy struct {
			ID string `json:"id"`
		} `json:"policy"`
	} `json:"data"`
}

func (c *ClientImpl) uploadChunk(file io.Reader, chuckID int, buff []byte, sessionID string) error {
	n, _ := io.ReadFull(file, buff)
	resp, err := c.client.R().
		SetContext(c.ctx).
		SetBody(buff[0:n]).
		ForceContentType("application/octet-stream").
		Post(c.url.JoinPath(fmt.Sprintf("api/v3/file/upload/%s/%d", sessionID, chuckID)).String())
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("upload chunk failed: %s, %s", resp.Status(), string(resp.Body()))
	}

	return nil
}

func (c *ClientImpl) getPolicyID() (string, error) {
	d, err := c.listDirectory("/")
	if err != nil {
		return "", err
	}

	return d.Data.Policy.ID, nil
}

func (c *ClientImpl) listDirectory(dir string) (*directory, error) {
	resp, err := c.client.R().
		SetContext(c.ctx).
		Get(c.url.JoinPath("api/v3/directory" + url.QueryEscape(dir)).String())
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("get list dir failed: %s, %s", resp.Status(), string(resp.Body()))
	}
	d := directory{}
	err = json.Unmarshal(resp.Body(), &d)
	if err != nil {
		return nil, fmt.Errorf("unmarshal policy failed: %w", err)
	}
	if d.Code != 0 {
		return nil, fmt.Errorf("err %d, %s", d.Code, d.Msg)
	}

	return &d, nil
}

func miteType(file string) string {
	t, err := mimetype.DetectFile(file)
	if err != nil {
		return "application/octet-stream"
	}
	return t.String()
}
