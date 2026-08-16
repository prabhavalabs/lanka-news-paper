import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@snap/ui/table'

export function SourcesPage() {
  return (
    <section>
      <h1 className="text-xl font-medium">Sources</h1>
      <p className="mt-2 text-sm text-[color:var(--ink-tertiary)]">
        No publishers onboarded. Rights profiles are required before public publishing.
      </p>
      <div className="mt-6">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Type</TableHead>
              <TableHead>Rights</TableHead>
              <TableHead>Health</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow>
              <TableCell colSpan={4} className="py-8 text-center text-[color:var(--ink-tertiary)]">
                Empty
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
    </section>
  )
}
