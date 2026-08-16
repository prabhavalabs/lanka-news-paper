export function getPaginationPages(page: number, totalPages: number) {
  return [...new Set([1, page - 1, page, page + 1, totalPages])]
    .filter((candidate) => candidate >= 1 && candidate <= totalPages)
    .sort((left, right) => left - right)
}
