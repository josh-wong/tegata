import { DayPicker } from "react-day-picker"
import { ChevronLeft, ChevronRight } from "lucide-react"
import { cn } from "@/lib/utils"

type CalendarProps = React.ComponentProps<typeof DayPicker>

function Calendar({ className, classNames, showOutsideDays = true, ...props }: CalendarProps) {
  return (
    <DayPicker
      showOutsideDays={showOutsideDays}
      className={cn("p-3 relative", className)}
      classNames={{
        months: "flex flex-col gap-4",
        month: "flex flex-col gap-4",
        month_caption: "relative flex items-center justify-center pt-1",
        caption_label: "text-sm font-medium",
        nav: "absolute inset-x-1 flex justify-between z-10",
        button_previous: cn(
          "inline-flex h-7 w-7 items-center justify-center rounded-md border border-input",
          "bg-background hover:bg-accent hover:text-accent-foreground opacity-50 hover:opacity-100",
        ),
        button_next: cn(
          "inline-flex h-7 w-7 items-center justify-center rounded-md border border-input",
          "bg-background hover:bg-accent hover:text-accent-foreground opacity-50 hover:opacity-100",
        ),
        month_grid: "w-full border-collapse",
        weekdays: "flex",
        weekday: "w-8 text-center text-[0.8rem] font-normal text-muted-foreground",
        week: "mt-2 flex w-full",
        day: "relative flex h-8 w-8 items-center justify-center p-0 text-sm",
        day_button: cn(
          "h-8 w-8 rounded-md font-normal",
          "hover:bg-accent hover:text-accent-foreground",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        ),
        selected:
          "rounded-md !bg-[var(--cinnabar)] text-white hover:!bg-[var(--cinnabar)] hover:text-white",
        today: "bg-accent text-accent-foreground",
        outside: "opacity-40",
        disabled: "opacity-30 pointer-events-none",
        hidden: "invisible",
        ...classNames,
      }}
      components={{
        Chevron: ({ orientation }) =>
          orientation === "left" ? (
            <ChevronLeft className="h-4 w-4" />
          ) : (
            <ChevronRight className="h-4 w-4" />
          ),
      }}
      {...props}
    />
  )
}

export { Calendar }
