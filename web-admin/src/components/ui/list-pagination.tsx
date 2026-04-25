import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

type ListPaginationProps = {
  page: number
  onPageChange: (page: number) => void
  pageSize: number
  onPageSizeChange: (pageSize: number) => void
  hasNextPage: boolean
  disabled?: boolean
}

const pageSizeOptions = [10, 25, 50, 100] as const

export function ListPagination({
  page,
  onPageChange,
  pageSize,
  onPageSizeChange,
  hasNextPage,
  disabled = false,
}: ListPaginationProps) {
  const { t } = useTranslation()

  return (
    <div className="flex flex-col gap-2 rounded-lg border bg-muted/10 px-3 py-2 text-sm md:flex-row md:items-center md:justify-between">
      <p className="text-muted-foreground">
        {t("listPagination.page", {
          defaultValue: "Page {{page}}",
          page,
        })}
      </p>
      <div className="flex flex-wrap items-center gap-2">
        <Select
          value={String(pageSize)}
          onValueChange={(value) => onPageSizeChange(Number.parseInt(value, 10) || 25)}
          disabled={disabled}
        >
          <SelectTrigger className="w-[120px]">
            <SelectValue placeholder={t("listPagination.pageSizePlaceholder", { defaultValue: "Page size" })} />
          </SelectTrigger>
          <SelectContent>
            {pageSizeOptions.map((item) => (
              <SelectItem key={item} value={String(item)}>
                {t("listPagination.pageSizeItem", {
                  defaultValue: "{{item}} / page",
                  item,
                })}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={disabled || page <= 1}
          onClick={() => onPageChange(Math.max(1, page - 1))}
        >
          {t("listPagination.prev", { defaultValue: "Previous" })}
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={disabled || !hasNextPage}
          onClick={() => onPageChange(page + 1)}
        >
          {t("listPagination.next", { defaultValue: "Next" })}
        </Button>
      </div>
    </div>
  )
}
