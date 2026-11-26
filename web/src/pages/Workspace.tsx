import { useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { Download, FilePlus, FileText, FolderIcon, FolderPlus, Pencil, RefreshCw, Trash2, Upload } from 'lucide-react'
import { toast } from 'sonner'
import {
  deleteWorkspacePath,
  fileUrl,
  joinPath,
  listWorkspace,
  makeWorkspaceDir,
  readWorkspaceFile,
  writeWorkspaceFile,
  type WorkspaceEntry,
} from '../lib/api/workspace'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '../components/ui/alert-dialog'
import { Button } from '../components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../components/ui/dialog'
import { Input } from '../components/ui/input'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '../components/ui/sheet'
import { Skeleton } from '../components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table'
import { Textarea } from '../components/ui/textarea'

const MAX_PREVIEW_BYTES = 2 * 1024 * 1024

function formatSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let value = bytes / 1024
  let i = 0
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024
    i++
  }
  return `${value.toFixed(value < 10 ? 1 : 0)} ${units[i]}`
}

function FileSheet({ path, size, onClose, onChanged }: { path: string; size: number; onClose: () => void; onChanged: () => void }) {
  const tooLarge = size > MAX_PREVIEW_BYTES
  const { data, isLoading, error } = useQuery({
    queryKey: ['workspace-file', path],
    queryFn: () => readWorkspaceFile(path),
    enabled: !tooLarge,
    staleTime: 0,
    gcTime: 0,
  })
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')
  const [confirmDelete, setConfirmDelete] = useState(false)

  const save = useMutation({
    mutationFn: () => writeWorkspaceFile(path, draft),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['workspace-file', path] })
      onChanged()
      setEditing(false)
      toast.success('File saved')
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const remove = useMutation({
    mutationFn: () => deleteWorkspacePath(path),
    onSuccess: () => {
      onChanged()
      toast.success('File deleted')
      onClose()
    },
    onError: (err: Error) => toast.error(err.message),
  })

  return (
    <>
      <Sheet open onOpenChange={open => !open && onClose()}>
        <SheetContent className="w-full sm:max-w-2xl gap-0">
          <SheetHeader>
            <SheetTitle className="font-mono text-sm break-all">{path}</SheetTitle>
            <SheetDescription>{formatSize(size)}</SheetDescription>
          </SheetHeader>
          <div className="flex flex-wrap items-center gap-2 px-4 pb-3">
            {editing ? (
              <>
                <Button size="sm" onClick={() => save.mutate()} disabled={save.isPending}>
                  Save
                </Button>
                <Button size="sm" variant="outline" onClick={() => setEditing(false)}>
                  Cancel
                </Button>
              </>
            ) : (
              <Button
                size="sm"
                variant="outline"
                disabled={data === undefined}
                onClick={() => {
                  setDraft(data ?? '')
                  setEditing(true)
                }}
              >
                <Pencil /> Edit
              </Button>
            )}
            <Button size="sm" variant="outline" asChild>
              <a href={fileUrl(path, true)} download>
                <Download /> Download
              </a>
            </Button>
            <Button size="sm" variant="destructive" className="ml-auto" onClick={() => setConfirmDelete(true)}>
              <Trash2 /> Delete
            </Button>
          </div>
          <div className="min-h-0 flex-1 overflow-auto px-4 pb-4">
            {tooLarge ? (
              <p className="text-sm text-muted-foreground italic">File is too large to preview. Use Download.</p>
            ) : error ? (
              <p className="text-sm text-destructive">Failed to load file: {error.message}</p>
            ) : isLoading ? (
              <Skeleton className="h-48 rounded-xl" />
            ) : editing ? (
              <Textarea
                value={draft}
                onChange={e => setDraft(e.target.value)}
                spellCheck={false}
                className="h-full min-h-96 font-mono text-xs"
              />
            ) : (
              <pre className="whitespace-pre-wrap break-all rounded-xl bg-muted/40 p-3 font-mono text-xs">{data}</pre>
            )}
          </div>
        </SheetContent>
      </Sheet>
      <AlertDialog open={confirmDelete} onOpenChange={setConfirmDelete}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete file?</AlertDialogTitle>
            <AlertDialogDescription>
              <span className="font-mono">{path}</span> will be permanently removed.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={() => remove.mutate()}>
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

export default function Workspace() {
  const queryClient = useQueryClient()
  const [params, setParams] = useSearchParams()
  const dir = params.get('path') ?? ''
  const segments = dir ? dir.split('/') : []
  const uploadRef = useRef<HTMLInputElement>(null)

  const [openFile, setOpenFile] = useState<{ path: string; size: number } | null>(null)
  const [creating, setCreating] = useState<'file' | 'folder' | null>(null)
  const [newName, setNewName] = useState('')
  const [deleting, setDeleting] = useState<WorkspaceEntry | null>(null)

  const queryKey = ['workspace', dir]
  const { data, isLoading, isFetching, error, refetch } = useQuery({ queryKey, queryFn: () => listWorkspace(dir) })

  useEffect(() => {
    setOpenFile(null)
  }, [dir])

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['workspace'] })

  const navigate = (path: string) => setParams(path ? { path } : {})

  const create = useMutation({
    mutationFn: async ({ kind, name }: { kind: 'file' | 'folder'; name: string }) => {
      const path = joinPath(dir, name)
      if (kind === 'folder') await makeWorkspaceDir(path)
      else await writeWorkspaceFile(path, '')
      return { kind, path }
    },
    onSuccess: ({ kind, path }) => {
      refresh()
      setCreating(null)
      setNewName('')
      if (kind === 'file') setOpenFile({ path, size: 0 })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const upload = useMutation({
    mutationFn: async (files: File[]) => {
      for (const file of files) await writeWorkspaceFile(joinPath(dir, file.name), file)
      return files.length
    },
    onSuccess: n => {
      refresh()
      toast.success(`Uploaded ${n} file${n === 1 ? '' : 's'}`)
    },
    onError: (err: Error) => {
      refresh()
      toast.error(err.message)
    },
  })

  const remove = useMutation({
    mutationFn: (entry: WorkspaceEntry) => deleteWorkspacePath(joinPath(dir, entry.name)),
    onSuccess: () => {
      refresh()
      toast.success('Deleted')
      setDeleting(null)
    },
    onError: (err: Error) => {
      toast.error(err.message)
      setDeleting(null)
    },
  })

  const entries = [...(data ?? [])].sort((a, b) =>
    a.type === b.type ? a.name.localeCompare(b.name) : a.type === 'dir' ? -1 : 1,
  )

  return (
    <div className="p-4 md:p-6 space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-foreground">Workspace</h1>
          <p className="text-sm text-muted-foreground">Browse and edit files in the <span className="font-mono">workspace</span> folder of the data directory.</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button size="sm" onClick={() => setCreating('file')}>
            <FilePlus /> New File
          </Button>
          <Button size="sm" variant="outline" onClick={() => setCreating('folder')}>
            <FolderPlus /> New Folder
          </Button>
          <Button size="sm" variant="outline" onClick={() => uploadRef.current?.click()} disabled={upload.isPending}>
            <Upload /> Upload
          </Button>
          <input
            ref={uploadRef}
            type="file"
            multiple
            className="hidden"
            onChange={e => {
              const files = Array.from(e.target.files ?? [])
              e.target.value = ''
              if (files.length) upload.mutate(files)
            }}
          />
          <Button size="icon-sm" variant="outline" aria-label="Refresh" onClick={() => refetch()}>
            <RefreshCw className={isFetching ? 'animate-spin' : ''} />
          </Button>
        </div>
      </div>

      <nav className="flex flex-wrap items-center gap-1 font-mono text-xs" aria-label="Path">
        <button type="button" className="text-primary hover:underline" onClick={() => navigate('')}>
          ~
        </button>
        {segments.map((seg, i) => (
          <span key={i} className="flex items-center gap-1">
            <span className="text-muted-foreground">/</span>
            {i === segments.length - 1 ? (
              <span>{seg}</span>
            ) : (
              <button
                type="button"
                className="text-primary hover:underline"
                onClick={() => navigate(segments.slice(0, i + 1).join('/'))}
              >
                {seg}
              </button>
            )}
          </span>
        ))}
      </nav>

      {error && <p className="text-sm text-destructive">Failed to load directory: {error.message}</p>}

      {isLoading ? (
        <Skeleton className="h-48 rounded-xl" />
      ) : (
        <div className="rounded-xl bg-card ring-1 ring-foreground/10 overflow-hidden">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead className="w-28 text-right">Size</TableHead>
                <TableHead className="w-48">Modified</TableHead>
                <TableHead className="w-12" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {entries.length === 0 && (
                <TableRow>
                  <TableCell colSpan={4} className="text-xs text-muted-foreground italic">
                    This directory is empty.
                  </TableCell>
                </TableRow>
              )}
              {entries.map(entry => {
                const path = joinPath(dir, entry.name)
                const isDir = entry.type === 'dir'
                return (
                  <TableRow key={entry.name}>
                    <TableCell className="py-1">
                      <button
                        type="button"
                        className="flex items-center gap-2 text-left text-xs hover:underline"
                        onClick={() => (isDir ? navigate(path) : setOpenFile({ path, size: entry.size }))}
                      >
                        {isDir ? (
                          <FolderIcon className="size-3.5 shrink-0 text-primary" />
                        ) : (
                          <FileText className="size-3.5 shrink-0 text-muted-foreground" />
                        )}
                        <span className="font-mono">{entry.name}</span>
                      </button>
                    </TableCell>
                    <TableCell className="py-1 text-right text-xs tabular-nums text-muted-foreground">
                      {isDir ? '' : formatSize(entry.size)}
                    </TableCell>
                    <TableCell className="py-1 text-xs tabular-nums text-muted-foreground">
                      {new Date(entry.modified).toLocaleString()}
                    </TableCell>
                    <TableCell className="py-1 text-right">
                      <Button size="icon-xs" variant="ghost" aria-label={`Delete ${entry.name}`} onClick={() => setDeleting(entry)}>
                        <Trash2 />
                      </Button>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>
      )}

      {openFile && (
        <FileSheet
          key={openFile.path}
          path={openFile.path}
          size={openFile.size}
          onClose={() => setOpenFile(null)}
          onChanged={refresh}
        />
      )}

      <Dialog
        open={creating !== null}
        onOpenChange={open => {
          if (!open) {
            setCreating(null)
            setNewName('')
          }
        }}
      >
        <DialogContent>
          <form
            className="space-y-4"
            onSubmit={e => {
              e.preventDefault()
              const name = newName.trim()
              if (creating && name) create.mutate({ kind: creating, name })
            }}
          >
            <DialogHeader>
              <DialogTitle>{creating === 'folder' ? 'New Folder' : 'New File'}</DialogTitle>
            </DialogHeader>
            <Input
              autoFocus
              value={newName}
              onChange={e => setNewName(e.target.value)}
              placeholder={creating === 'folder' ? 'folder-name' : 'notes.txt'}
              className="font-mono"
            />
            <DialogFooter>
              <Button type="submit" disabled={!newName.trim() || create.isPending}>
                Create
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <AlertDialog open={deleting !== null} onOpenChange={open => !open && setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete {deleting?.type === 'dir' ? 'folder' : 'file'}?</AlertDialogTitle>
            <AlertDialogDescription>
              <span className="font-mono">{deleting?.name}</span> will be permanently removed.
              {deleting?.type === 'dir' && ' Only empty folders can be deleted.'}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={() => deleting && remove.mutate(deleting)}>
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
