//! The cases on NSH, written in Rust: each binary of src/bin mounts one of
//! these roots on the machine's own backend.

use nsh_core::prelude::*;

#[cfg(target_os = "linux")]
use nsh_platform_linux as platform;
#[cfg(target_os = "macos")]
use nsh_platform_macos as platform;
#[cfg(target_os = "windows")]
use nsh_platform_windows as platform;

/// Opens a window of `title` and `size` showing the root `R`.
pub fn run<R: Component<Props: Default> + 'static>(title: &str, size: (i32, i32)) {
    let _ = platform::run_app(
        platform::AppSource::Rust(platform::Root::of::<R>()),
        platform::AppOptions {
            title: title.to_owned(),
            #[cfg(any(target_os = "linux", target_os = "windows"))]
            app_id: format!("dev.oneo.nsgo.benchmark.{}", title.to_lowercase()),
            size,
            ..Default::default()
        },
    );
}

const INK: u32 = 0x111111;
const MUTED: u32 = 0x6b7280;
const UP: u32 = 0x1a7f37;
const DOWN: u32 = 0xcf222e;
const ZEBRA: u32 = 0xf6f7f9;
const RULE: u32 = 0xe5e7eb;
const WHITE: u32 = 0xffffff;

fn tone(v: f64) -> Rgba {
    rgb(if v < 0.0 { DOWN } else { UP })
}

/// A button: a label on a rounded fill that dims while pressed.
fn button(label: &'static str, fill: u32, ink: u32, press: impl Fn() + 'static) -> impl IntoElement {
    view()
        .px(px(14.0))
        .py(px(6.0))
        .rounded(px(6.0))
        .bg(rgb(fill))
        .active(|s| s.opacity(0.7))
        .on_press(press)
        .child(text(label).text_size(px(14.0)).font_weight(FontWeight::MEDIUM).text_color(rgb(ink)))
}

/// A count and two buttons.
#[component]
pub fn Counter() -> impl IntoElement {
    let count = signal(0_i32);
    // The first frame after the first commit is ready.
    request_animation_frame(|_| cases::frame());
    view()
        .flex_1()
        .bg(rgb(WHITE))
        .items_center()
        .justify_center()
        .gap(px(16.0))
        .child(
            text(move || count.get().to_string())
                .text_size(px(56.0))
                .font_weight(FontWeight::BOLD)
                .text_color(rgb(INK)),
        )
        .child(
            view()
                .flex_row()
                .gap(px(8.0))
                .child(button("−1", 0xf4f4f5, INK, move || count.update(|n| *n -= 1)))
                .child(button("+1", 0x007aff, WHITE, move || count.update(|n| *n += 1))),
        )
}

const WIDTHS: [f32; 5] = [80.0, 90.0, 80.0, 80.0, 100.0];

/// Asks for the next frame, which runs `f` and asks again.
fn every_frame(f: impl Fn() + 'static) {
    let f = std::rc::Rc::new(f);
    fn next(f: std::rc::Rc<dyn Fn()>) {
        request_animation_frame(move |_| {
            f();
            next(f);
        });
    }
    next(f);
}

/// A table of 40 quotes, ten of which change every display frame.
#[component]
pub fn Quotes() -> impl IntoElement {
    let market = std::rc::Rc::new(std::cell::RefCell::new(cases::Quotes::new()));
    // A row is one signal: a tick sets the rows it changed, once each.
    let rows: Vec<_> = market.borrow().rows.iter().map(|q| signal(*q)).collect();
    let ticks = signal(0_u64);
    {
        let rows = rows.clone();
        let market = market.clone();
        every_frame(move || {
            cases::frame();
            let mut m = market.borrow_mut();
            for i in m.tick() {
                rows[i].set(m.rows[i]);
            }
            ticks.update(|n| *n += 1);
        });
    }
    let header = |label: &'static str, col: usize| {
        let t = text(label).w(px(WIDTHS[col])).text_size(px(12.0)).text_color(rgb(MUTED));
        if col == 0 { t } else { t.text_right() }
    };
    let cell = move |row: Signal<cases::Quote>, col: usize, color: fn(&cases::Quote) -> Rgba, value: fn(&cases::Quote) -> String| {
        text(move || value(&row.get()))
            .w(px(WIDTHS[col]))
            .text_right()
            .text_size(px(13.0))
            .text_color(move || color(&row.get()))
    };
    let mut table = view().gap(px(cases::QUOTE_GAP));
    for (i, row) in rows.iter().copied().enumerate() {
        let symbol = market.borrow().rows[i].symbol().to_owned();
        table = table.child(
            view()
                .flex_row()
                .gap(px(12.0))
                .h(px(cases::QUOTE_ROW_H))
                .items_center()
                .rounded(px(4.0))
                .bg(rgb(if i % 2 == 1 { ZEBRA } else { WHITE }))
                .child(text(symbol).w(px(WIDTHS[0])).text_size(px(13.0)).font_weight(FontWeight::SEMIBOLD).text_color(rgb(INK)))
                .child(cell(row, 1, |_| rgb(INK), |q| format!("{:.2}", q.price)))
                .child(cell(row, 2, |q| tone(q.change()), |q| format!("{:.2}", q.change())))
                .child(cell(row, 3, |q| tone(q.change()), |q| format!("{:.2}%", q.percent())))
                .child(cell(row, 4, |_| rgb(MUTED), |q| q.volume.to_string())),
        );
    }
    view()
        .flex_1()
        .bg(rgb(WHITE))
        .p(px(16.0))
        .gap(px(cases::QUOTE_GAP))
        .child(
            text(move || format!("Streaming quotes · tick {}", ticks.get()))
                .text_size(px(18.0))
                .font_weight(FontWeight::SEMIBOLD)
                .text_color(rgb(INK)),
        )
        .child(
            view()
                .flex_row()
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
        .child(table)
}

const TONES: [u32; 4] = [0x007aff, 0x34c759, 0xff9500, 0xaf52de];

/// 10,000 rows, scrolled a row further every display frame.
#[component]
pub fn List() -> impl IntoElement {
    let list = element_ref();
    let top = std::cell::Cell::new(0_u32);
    every_frame(move || {
        cases::frame();
        top.set((top.get() + 1) % (cases::LIST_ROWS as u32 - 20));
        list.scroll_to_index(top.get(), percentage(0.0), px(0.0), false);
    });
    virtual_list(cases::LIST_ROWS, |at| {
        view()
            .flex_row()
            .h(px(cases::LIST_ROW_H))
            .items_center()
            .gap(px(12.0))
            .px(px(16.0))
            .child(view().w(px(10.0)).h(px(10.0)).rounded(px(5.0)).bg(move || rgb(TONES[at.index() % TONES.len()])))
            .child(text(move || format!("Row {}", at.index())).flex_1().text_size(px(14.0)))
            .child(text(move || (at.index() * 7 % 1000).to_string()).text_size(px(13.0)).text_color(rgb(MUTED)))
    })
    .estimated_item_size(px(cases::LIST_ROW_H))
    .flex_1()
    .bg(rgb(WHITE))
    .element_ref(&list)
}
