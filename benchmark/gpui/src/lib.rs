//! The cases on GPUI Kit, which builds on upstream GPUI's snapshot, or with
//! the feature `fast` on GPUI Fast: each binary of src/bin opens one of
//! these views.

extern crate gpui_kit as gpui;


use gpui_kit::component::button::*;
use gpui_kit::*;

/// Opens a window of `title` and `size` showing the view `build` makes.
pub fn run<V: Render>(title: &'static str, size: (f32, f32), build: impl FnOnce(&mut Window, &mut App) -> V + 'static) {
    gpui_kit::application().run(move |cx| {
        gpui_kit::init(cx);
        let bounds = Bounds::centered(None, gpui_kit::size(px(size.0), px(size.1)), cx);
        let options = WindowOptions {
            window_bounds: Some(WindowBounds::Windowed(bounds)),
            titlebar: Some(TitlebarOptions { title: Some(title.into()), ..Default::default() }),
            ..Default::default()
        };
        gpui_kit::open_window(options, cx, |window, cx| {
            let view = build(window, cx);
            cx.new(|_| view)
        })
        .expect("open the window");
        cx.activate(true);
    });
}

const INK: u32 = 0x111111;
const MUTED: u32 = 0x6b7280;
const UP: u32 = 0x1a7f37;
const DOWN: u32 = 0xcf222e;
const ZEBRA: u32 = 0xf6f7f9;
const RULE: u32 = 0xe5e7eb;
const WHITE: u32 = 0xffffff;

fn tone(v: f64) -> Hsla {
    rgb(if v < 0.0 { DOWN } else { UP }).into()
}

/// A count and two buttons.
#[derive(Default)]
pub struct Counter {
    count: i32,
    rendered: bool,
}

impl Render for Counter {
    fn render(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        // The frame after the first render is ready.
        if !self.rendered {
            self.rendered = true;
            window.on_next_frame(|_, _| cases::frame());
        }
        div()
            .size_full()
            .bg(rgb(WHITE))
            .flex()
            .flex_col()
            .items_center()
            .justify_center()
            .gap(px(16.0))
            .child(div().text_size(px(56.0)).font_weight(FontWeight::BOLD).text_color(rgb(INK)).child(self.count.to_string()))
            .child(
                div()
                    .flex()
                    .gap(px(8.0))
                    .child(Button::new("decrement").label("−1").on_click(cx.listener(|this, _, _, cx| {
                        this.count -= 1;
                        cx.notify();
                    })))
                    .child(Button::new("increment").primary().label("+1").on_click(cx.listener(|this, _, _, cx| {
                        this.count += 1;
                        cx.notify();
                    }))),
            )
    }
}

const WIDTHS: [f32; 5] = [80.0, 90.0, 80.0, 80.0, 100.0];

/// A table of the quotes, a quarter of which change every display frame.
pub struct Quotes {
    market: cases::Quotes,
    ticks: u64,
    rendered: bool,
}

impl Default for Quotes {
    fn default() -> Self {
        Quotes { market: cases::Quotes::new(), ticks: 0, rendered: false }
    }
}

fn cell(col: usize, color: impl Into<Hsla>, value: String) -> Div {
    div().w(px(WIDTHS[col])).flex().justify_end().text_size(px(13.0)).text_color(color).child(value)
}

impl Render for Quotes {
    fn render(&mut self, window: &mut Window, _: &mut Context<Self>) -> impl IntoElement {
        // Each frame ticks the market and asks for the next; the first
        // render comes before the first frame, which is ready.
        if self.rendered {
            cases::frame();
        }
        self.rendered = true;
        self.market.tick();
        self.ticks += 1;
        window.request_animation_frame();
        let header = |label: &'static str, col: usize| {
            let d = div().w(px(WIDTHS[col])).flex().text_size(px(12.0)).text_color(rgb(MUTED));
            if col == 0 { d.child(label) } else { d.justify_end().child(label) }
        };
        div()
            .size_full()
            .bg(rgb(WHITE))
            .p(px(16.0))
            .flex()
            .flex_col()
            .gap(px(cases::QUOTE_GAP))
            .child(
                div()
                    .text_size(px(18.0))
                    .font_weight(FontWeight::SEMIBOLD)
                    .text_color(rgb(INK))
                    .child(format!("Streaming quotes · tick {}", self.ticks)),
            )
            .child(
                div()
                    .flex()
                    .gap(px(12.0))
                    .h(px(24.0))
                    .items_center()
                    .border_1()
                    .border_color(rgb(RULE))
                    .child(header("Symbol", 0))
                    .child(header("Price", 1))
                    .child(header("Change", 2))
                    .child(header("%", 3))
                    .child(header("Volume", 4)),
            )
            .children(self.market.rows.iter().enumerate().map(|(i, q)| {
                div()
                    .flex()
                    .gap(px(12.0))
                    .h(px(cases::QUOTE_ROW_H))
                    .items_center()
                    .rounded(px(4.0))
                    .bg(rgb(if i % 2 == 1 { ZEBRA } else { WHITE }))
                    .child(
                        div()
                            .w(px(WIDTHS[0]))
                            .text_size(px(13.0))
                            .font_weight(FontWeight::SEMIBOLD)
                            .text_color(rgb(INK))
                            .child(q.symbol().to_owned()),
                    )
                    .child(cell(1, rgb(INK), format!("{:.2}", q.price)))
                    .child(cell(2, tone(q.change()), format!("{:.2}", q.change())))
                    .child(cell(3, tone(q.change()), format!("{:.2}%", q.percent())))
                    .child(cell(4, rgb(MUTED), q.volume.to_string()))
            }))
    }
}

const TONES: [u32; 4] = [0x007aff, 0x34c759, 0xff9500, 0xaf52de];

/// The rows, scrolled a row further every display frame.
#[derive(Default)]
pub struct List {
    scroll: UniformListScrollHandle,
    top: usize,
    rendered: bool,
}

impl Render for List {
    fn render(&mut self, window: &mut Window, _: &mut Context<Self>) -> impl IntoElement {
        if self.rendered {
            cases::frame();
        }
        self.rendered = true;
        self.top = (self.top + 1) % (cases::LIST_ROWS - 20);
        self.scroll.scroll_to_item_strict(self.top, ScrollStrategy::Top);
        window.request_animation_frame();
        div().size_full().bg(rgb(WHITE)).child(
            uniform_list("rows", cases::LIST_ROWS, |range, _, _| {
                range
                    .map(|i| {
                        div()
                            .flex()
                            .w_full()
                            .h(px(cases::LIST_ROW_H))
                            .items_center()
                            .gap(px(12.0))
                            .px(px(16.0))
                            .child(div().size(px(10.0)).rounded(px(5.0)).bg(rgb(TONES[i % TONES.len()])))
                            .child(div().flex_1().text_size(px(14.0)).text_color(rgb(INK)).child(format!("Row {i}")))
                            .child(div().text_size(px(13.0)).text_color(rgb(MUTED)).child((i * 7 % 1000).to_string()))
                    })
                    .collect()
            })
            .track_scroll(&self.scroll)
            .size_full(),
        )
    }
}

