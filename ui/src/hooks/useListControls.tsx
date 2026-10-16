import { useEffect, useMemo, useRef, useState } from 'react'
import { SearchInput } from '@/components/ui/search-input'
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationNext,
  PaginationPrevious,
} from '@/components/ui/pagination'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

export function useDebouncedValue<T>(value: T, delay = 250): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setDebounced(value), delay)
    return () => clearTimeout(t)
  }, [value, delay])
  return debounced
}

export function useListControls<T>(
  items: T[] | undefined,
  options: {
    getText: (item: T) => string
    pageSize?: number
  },
) {
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(options.pageSize ?? 10)
  const debouncedQuery = useDebouncedValue(query)
  const getTextRef = useRef(options.getText)
  getTextRef.current = options.getText

  const filtered = useMemo(() => {
    const list = items ?? []
    const q = debouncedQuery.trim().toLowerCase()
    if (!q) return list
    return list.filter((item) => getTextRef.current(item).toLowerCase().includes(q))
  }, [items, debouncedQuery])

  const total = filtered.length
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const safePage = Math.min(page, totalPages)
  const start = (safePage - 1) * pageSize
  const pageItems = filtered.slice(start, start + pageSize)

  useEffect(() => {
    setPage(1)
  }, [debouncedQuery, pageSize])

  return {
    query,
    setQuery,
    page: safePage,
    setPage,
    pageSize,
    setPageSize,
    total,
    totalPages,
    pageItems,
    filtered,
    rangeLabel:
      total === 0 ? '0 of 0' : `${start + 1}–${Math.min(start + pageSize, total)} of ${total}`,
  }
}

export function ListToolbar({
  query,
  onQueryChange,
  placeholder = 'Search...',
  className,
}: {
  query: string
  onQueryChange: (value: string) => void
  placeholder?: string
  className?: string
}) {
  return (
    <div className={className ?? 'mb-4 flex items-center gap-2'}>
      <SearchInput
        value={query}
        onChange={(e) => onQueryChange(e.target.value)}
        placeholder={placeholder}
        className="max-w-sm"
      />
    </div>
  )
}

export function ListPagination({
  page,
  totalPages,
  pageSize,
  total,
  rangeLabel,
  onPageChange,
  onPageSizeChange,
}: {
  page: number
  totalPages: number
  pageSize: number
  total: number
  rangeLabel: string
  onPageChange: (page: number) => void
  onPageSizeChange?: (size: number) => void
}) {
  if (total === 0) return null

  return (
    <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <span>{rangeLabel}</span>
        {onPageSizeChange && (
          <Select value={String(pageSize)} onValueChange={(v) => onPageSizeChange(Number(v))}>
            <SelectTrigger className="h-8 w-[7.5rem]" aria-label="Rows per page">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {[10, 20, 50].map((n) => (
                <SelectItem key={n} value={String(n)}>
                  {n} / page
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      </div>

      <Pagination>
        <PaginationContent>
          <PaginationItem>
            <PaginationPrevious
              type="button"
              disabled={page <= 1}
              onClick={() => page > 1 && onPageChange(page - 1)}
            />
          </PaginationItem>
          <PaginationItem>
            <span className="px-2 text-sm text-muted-foreground">
              Page {page} of {totalPages}
            </span>
          </PaginationItem>
          <PaginationItem>
            <PaginationNext
              type="button"
              disabled={page >= totalPages}
              onClick={() => page < totalPages && onPageChange(page + 1)}
            />
          </PaginationItem>
        </PaginationContent>
      </Pagination>
    </div>
  )
}
