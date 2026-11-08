import type { JSX } from "nsgo";
import { ButtonDemo, LinkDemo, ToolbarDemo } from "./demos/Actions";
import { AccordionDemo, DockDemo, ResizableDemo } from "./demos/Containers";
import { CommitGraphDemo, DataTableDemo, ListDemo, PlotDemo, TableDemo } from "./demos/Data";
import { AvatarDemo, BadgeDemo, CarouselDemo, ImageDemo } from "./demos/Display";
import { DiffDemo, EditorDemo, TextViewDemo } from "./demos/Documents";
import { ProgressDemo, ToastDemo } from "./demos/Feedback";
import { CheckboxDemo, ColorPickerDemo, DatePickerDemo, InputDemo, SelectDemo, SliderDemo } from "./demos/Inputs";
import { BreadcrumbsDemo, NavStackDemo, SidebarDemo, TabsDemo } from "./demos/Navigation";
import { CommandDemo, DialogDemo, MenuDemo, PopoverDemo } from "./demos/Popups";

export interface Demo {
  name: string;
  icon: string;
  screen: () => JSX.Element;
}

export interface Group {
  name: string;
  demos: Demo[];
}

/** The gallery's Components by what they do, as the Go gallery lists them. */
export const groups: Group[] = [
  {
    name: "Actions",
    demos: [
      { name: "Button", icon: "mouse-pointer-click", screen: ButtonDemo },
      { name: "Link", icon: "link", screen: LinkDemo },
      { name: "Toolbar", icon: "panel-top", screen: ToolbarDemo },
    ],
  },
  {
    name: "Inputs",
    demos: [
      { name: "Input", icon: "text-cursor-input", screen: InputDemo },
      { name: "Checkbox", icon: "square-check", screen: CheckboxDemo },
      { name: "Select", icon: "chevrons-up-down", screen: SelectDemo },
      { name: "DatePicker", icon: "calendar", screen: DatePickerDemo },
      { name: "ColorPicker", icon: "palette", screen: ColorPickerDemo },
      { name: "Slider", icon: "sliders-horizontal", screen: SliderDemo },
    ],
  },
  {
    name: "Display",
    demos: [
      { name: "Badge", icon: "tag", screen: BadgeDemo },
      { name: "Avatar", icon: "user", screen: AvatarDemo },
      { name: "Image", icon: "image", screen: ImageDemo },
      { name: "Carousel", icon: "gallery-horizontal", screen: CarouselDemo },
    ],
  },
  {
    name: "Data",
    demos: [
      { name: "List", icon: "list", screen: ListDemo },
      { name: "Table", icon: "table", screen: TableDemo },
      { name: "DataTable", icon: "table", screen: DataTableDemo },
      { name: "CommitGraph", icon: "git-graph", screen: CommitGraphDemo },
      { name: "Plot", icon: "chart-spline", screen: PlotDemo },
    ],
  },
  {
    name: "Containers",
    demos: [
      { name: "Accordion", icon: "rows-3", screen: AccordionDemo },
      { name: "Resizable", icon: "panel-left", screen: ResizableDemo },
      { name: "Dock", icon: "app-window", screen: DockDemo },
    ],
  },
  {
    name: "Feedback",
    demos: [
      { name: "Progress", icon: "loader-circle", screen: ProgressDemo },
      { name: "Toast", icon: "bell", screen: ToastDemo },
    ],
  },
  {
    name: "Popups",
    demos: [
      { name: "Dialog", icon: "app-window", screen: DialogDemo },
      { name: "Popover", icon: "message-square", screen: PopoverDemo },
      { name: "Command", icon: "search", screen: CommandDemo },
      { name: "Menu", icon: "menu", screen: MenuDemo },
    ],
  },
  {
    name: "Navigation",
    demos: [
      { name: "Tabs", icon: "notebook-tabs", screen: TabsDemo },
      { name: "Breadcrumbs", icon: "chevron-right", screen: BreadcrumbsDemo },
      { name: "Sidebar", icon: "panel-left", screen: SidebarDemo },
      { name: "NavStack", icon: "panels-top-left", screen: NavStackDemo },
    ],
  },
  {
    name: "Documents",
    demos: [
      { name: "Editor", icon: "code", screen: EditorDemo },
      { name: "DiffView", icon: "file-diff", screen: DiffDemo },
      { name: "TextView", icon: "file-text", screen: TextViewDemo },
    ],
  },
];

export const demos = groups.flatMap((g) => g.demos);
