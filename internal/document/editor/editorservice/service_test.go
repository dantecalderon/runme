package editorservice

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"

	parserv1 "github.com/stateful/runme/internal/gen/proto/go/runme/parser/v1"
	"github.com/stateful/runme/internal/identity"
	"github.com/stateful/runme/internal/version"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
)

var (
	testMockID = identity.GenerateID()
	client     parserv1.ParserServiceClient

	documentWithoutFrontmatter = strings.Join([]string{
		"# H1",
		"```sh { name=foo id=123 }",
		`echo "Foo"`,
		"```",
		"## H2",
		"```sh { name=bar }",
		`echo "Bar"`,
		"```",
		"### H3",
		"```js",
		`echo "Shebang"`,
		"```",
	}, "\n")

	documentWithFrontmatter = strings.Join([]string{
		"---",
		"prop: value",
		"runme:",
		"  id: 123",
		"  version: v1.0",
		"---",
		"",
		documentWithoutFrontmatter,
	}, "\n")
)

func TestMain(m *testing.M) {
	identity.MockGenerator(testMockID)

	lis := bufconn.Listen(2048)
	server := grpc.NewServer()
	parserv1.RegisterParserServiceServer(server, NewParserServiceServer(zap.NewNop()))
	go server.Serve(lis)

	conn, err := grpc.Dial(
		"",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		panic(err)
	}

	client = parserv1.NewParserServiceClient(conn)
	code := m.Run()

	identity.ResetGenerator()
	os.Exit(code)
}

func Test_IdentityUnspecified(t *testing.T) {
	tests := []struct {
		content             string
		hasExtraFrontmatter bool
	}{
		{content: documentWithFrontmatter, hasExtraFrontmatter: true},
		{content: documentWithoutFrontmatter, hasExtraFrontmatter: false},
	}

	for _, tt := range tests {
		identity := parserv1.RunmeIdentity_RUNME_IDENTITY_UNSPECIFIED

		dResp, err := deserialize(client, tt.content, identity)
		assert.NoError(t, err)

		rawFrontmatter, ok := dResp.Notebook.Metadata["runme.dev/frontmatter"]
		if tt.hasExtraFrontmatter {
			assert.True(t, ok)
			assert.Len(t, dResp.Notebook.Metadata, 2)
			assert.Contains(t, rawFrontmatter, "prop: value\n")
			assert.Contains(t, rawFrontmatter, "id: 123\n")
			assert.Contains(t, rawFrontmatter, "version: v1.0\n")
		} else {
			assert.False(t, ok)
			assert.Len(t, dResp.Notebook.Metadata, 1)
		}

		sResp, err := serialize(client, dResp.Notebook, identity)
		assert.NoError(t, err)
		content := string(sResp.Result)

		if tt.hasExtraFrontmatter {
			assert.Regexp(t, "^---\n", content)
		} else {
			assert.NotRegexp(t, "^---\n", content)
			assert.NotRegexp(t, "^\n\n", content)
		}

		assert.Contains(t, content, "```sh { name=foo id=123 }\n")
	}
}

func Test_IdentityAll(t *testing.T) {
	tests := []struct {
		content             string
		hasExtraFrontmatter bool
	}{
		{content: documentWithFrontmatter, hasExtraFrontmatter: true},
		{content: documentWithoutFrontmatter, hasExtraFrontmatter: false},
	}

	identity := parserv1.RunmeIdentity_RUNME_IDENTITY_ALL

	for _, tt := range tests {
		dResp, err := deserialize(client, tt.content, identity)
		assert.NoError(t, err)

		rawFrontmatter, ok := dResp.Notebook.Metadata["runme.dev/frontmatter"]
		assert.True(t, ok)

		assert.Len(t, dResp.Notebook.Metadata, 2)

		if tt.hasExtraFrontmatter {
			assert.Contains(t, rawFrontmatter, "prop: value")
		}

		assert.Contains(t, rawFrontmatter, "id: "+testMockID)
		assert.Contains(t, rawFrontmatter, "version: "+version.BaseVersion())

		sResp, err := serialize(client, dResp.Notebook, identity)
		assert.NoError(t, err)

		content := string(sResp.Result)
		assert.Regexp(t, "^---\n", content)
		assert.Contains(t, content, "runme:\n")
		assert.Contains(t, content, "id: "+testMockID)
		assert.Contains(t, content, "version: "+version.BaseVersion())
		assert.Contains(t, content, "```sh { name=foo id="+testMockID+" }\n")
		assert.Contains(t, content, "```sh { name=bar id="+testMockID+" }\n")
		assert.Contains(t, content, "```js { id="+testMockID+" }\n")
	}
}

func Test_IdentityDocument(t *testing.T) {
	tests := []struct {
		content             string
		hasExtraFrontmatter bool
	}{
		{content: documentWithFrontmatter, hasExtraFrontmatter: true},
		{content: documentWithoutFrontmatter, hasExtraFrontmatter: false},
	}

	identity := parserv1.RunmeIdentity_RUNME_IDENTITY_DOCUMENT

	for _, tt := range tests {
		dResp, err := deserialize(client, tt.content, identity)
		assert.NoError(t, err)

		rawFrontmatter, ok := dResp.Notebook.Metadata["runme.dev/frontmatter"]
		assert.True(t, ok)

		assert.Len(t, dResp.Notebook.Metadata, 2)

		if tt.hasExtraFrontmatter {
			assert.Contains(t, rawFrontmatter, "prop: value")
		}

		assert.Contains(t, rawFrontmatter, "id: "+testMockID)
		assert.Contains(t, rawFrontmatter, "version: "+version.BaseVersion())

		sResp, err := serialize(client, dResp.Notebook, identity)
		assert.NoError(t, err)

		content := string(sResp.Result)
		assert.Regexp(t, "^---\n", content)
		assert.Contains(t, content, "runme:\n")
		assert.Contains(t, content, "id: "+testMockID+"\n")
		assert.Contains(t, content, "version: "+version.BaseVersion()+"\n")
		assert.Contains(t, content, "```sh { name=foo id=123 }\n")
		assert.Contains(t, content, "```sh { name=bar }\n")
		assert.Contains(t, content, "```js\n")
	}
}

func Test_IdentityCell(t *testing.T) {
	tests := []struct {
		content             string
		hasExtraFrontmatter bool
	}{
		{content: documentWithFrontmatter, hasExtraFrontmatter: true},
		{content: documentWithoutFrontmatter, hasExtraFrontmatter: false},
	}

	identity := parserv1.RunmeIdentity_RUNME_IDENTITY_CELL

	for _, tt := range tests {
		dResp, err := deserialize(client, tt.content, identity)
		assert.NoError(t, err)

		rawFrontmatter, ok := dResp.Notebook.Metadata["runme.dev/frontmatter"]

		if tt.hasExtraFrontmatter {
			assert.True(t, ok)
			assert.Len(t, dResp.Notebook.Metadata, 2)
			assert.Contains(t, rawFrontmatter, "prop: value\n")
			assert.Contains(t, rawFrontmatter, "id: 123\n")
			assert.Contains(t, rawFrontmatter, "version: v1.0\n")
		} else {
			assert.False(t, ok)
			assert.Len(t, dResp.Notebook.Metadata, 1)
		}

		sResp, err := serialize(client, dResp.Notebook, identity)
		assert.NoError(t, err)

		content := string(sResp.Result)

		if tt.hasExtraFrontmatter {
			assert.Contains(t, content, "runme:\n")
			assert.Contains(t, content, "id: 123\n")
			assert.Contains(t, content, "version: v1.0\n")
		} else {
			assert.NotRegexp(t, "^---\n", content)
			assert.NotRegexp(t, "^\n\n", content)
		}

		assert.Contains(t, content, "```sh { name=foo id="+testMockID+" }\n")
		assert.Contains(t, content, "```sh { name=bar id="+testMockID+" }\n")
		assert.Contains(t, content, "```js { id="+testMockID+" }\n")
	}
}

func Test_parserServiceServer(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		os.Setenv("RUNME_AST_METADATA", "true")

		identity := parserv1.RunmeIdentity_RUNME_IDENTITY_UNSPECIFIED

		content := "# Title\n\nSome content with [Link1](https://example1.com \"Link title 1\") [Link2](https://example2.com \"Link title2\")"

		resp, err := deserialize(client, content, identity)

		cells := resp.Notebook.Cells
		assert.NoError(t, err)
		assert.Len(t, cells, 2)

		assert.True(
			t,
			proto.Equal(
				&parserv1.Cell{
					Kind:  parserv1.CellKind_CELL_KIND_MARKUP,
					Value: "# Title",
					Metadata: map[string]string{
						"runme.dev/ast": `{"Children":[{"Kind":"Text","Text":"Title"}],"Kind":"Heading","Level":1,"RawText":"Title"}`,
					},
				},
				cells[0],
			),
		)
		assert.True(
			t,
			proto.Equal(
				&parserv1.Cell{
					Kind:  parserv1.CellKind_CELL_KIND_MARKUP,
					Value: "Some content with [Link1](https://example1.com \"Link title 1\") [Link2](https://example2.com \"Link title2\")",
					Metadata: map[string]string{
						"runme.dev/ast": `{"Children":[{"Kind":"Text","Text":"Some content with "},{"Children":[{"Kind":"Text","Text":"Link1"}],"Destination":"https://example1.com","Kind":"Link","Title":"Link title 1"},{"Kind":"Text","Text":" "},{"Children":[{"Kind":"Text","Text":"Link2"}],"Destination":"https://example2.com","Kind":"Link","Title":"Link title2"}],"Kind":"Paragraph","RawText":"Some content with [Link1](https://example1.com \"Link title 1\") [Link2](https://example2.com \"Link title2\")"}`,
					},
				},
				cells[1],
			),
		)
	})
}

func deserialize(client parserv1.ParserServiceClient, content string, idt parserv1.RunmeIdentity) (*parserv1.DeserializeResponse, error) {
	return client.Deserialize(
		context.Background(),
		&parserv1.DeserializeRequest{
			Source: []byte(content),
			Options: &parserv1.DeserializeRequestOptions{
				Identity: idt,
			},
		},
	)
}

func serialize(client parserv1.ParserServiceClient, notebook *parserv1.Notebook, idt parserv1.RunmeIdentity) (*parserv1.SerializeResponse, error) {
	return client.Serialize(
		context.Background(),
		&parserv1.SerializeRequest{
			Notebook: notebook,
			Options: &parserv1.SerializeRequestOptions{
				Identity: idt,
			},
		},
	)
}
