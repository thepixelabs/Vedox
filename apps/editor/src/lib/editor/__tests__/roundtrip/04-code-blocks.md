## Code Blocks

A plain fenced code block with no language:

```
plain text block
no syntax highlighting
```

A TypeScript example:

```typescript
interface DocStore {
  get(id: string): Promise<Document | null>;
  put(doc: Document): Promise<void>;
  delete(id: string): Promise<void>;
  list(prefix: string): Promise<Document[]>;
}

async function loadDocument(store: DocStore, id: string): Promise<string> {
  const doc = await store.get(id);
  if (!doc) throw new Error(`VDX-404: document not found: ${id}`);
  return doc.content;
}
```

A Go snippet:

```go
func (s *LocalAdapter) Put(ctx context.Context, doc *Document) error {
    tmp, err := os.CreateTemp(s.root, ".vedox-tmp-*")
    if err != nil {
        return fmt.Errorf("VDX-501: %w", err)
    }
    defer os.Remove(tmp.Name())
    if _, err := tmp.WriteString(doc.Content); err != nil {
        return err
    }
    if err := tmp.Sync(); err != nil {
        return err
    }
    tmp.Close()
    return os.Rename(tmp.Name(), filepath.Join(s.root, doc.ID+".md"))
}
```

A shell example:

```bash
pnpm install
turbo run build --filter=@vedox/editor
go build ./...
```
