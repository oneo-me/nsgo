//! What the Rust programs of the benchmark share: the lines they print for
//! the runner and the cases' data, which the Go programs' package
//! (internal/bench) holds the same.

use std::cell::RefCell;
use std::time::Instant;

pub const QUOTE_ROWS: usize = 40;
pub const QUOTE_ROW_H: f32 = 18.0;
pub const QUOTE_GAP: f32 = 1.0;
pub const LIST_ROWS: usize = 10_000;
pub const LIST_ROW_H: f32 = 40.0;

thread_local! {
    static REPORT: RefCell<Option<(u32, Instant)>> = const { RefCell::new(None) };
}

/// Counts a frame that did the case's work. The first prints
/// "bench ready", and then each second "bench frames N", the frames of
/// that second.
pub fn frame() {
    let now = Instant::now();
    REPORT.with_borrow_mut(|report| match report {
        None => {
            *report = Some((0, now));
            println!("bench ready");
        }
        Some((frames, since)) => {
            *frames += 1;
            let d = now.duration_since(*since).as_secs_f64();
            if d >= 1.0 {
                println!("bench frames {:.1}", *frames as f64 / d);
                *frames = 0;
                *since = now;
            }
        }
    });
}

/// A row of the quotes case.
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Quote {
    symbol: [u8; 4],
    pub open: f64,
    pub price: f64,
    pub volume: i64,
}

impl Quote {
    pub fn symbol(&self) -> &str {
        std::str::from_utf8(&self.symbol).unwrap_or("")
    }

    /// The price's change since the open.
    pub fn change(&self) -> f64 {
        self.price - self.open
    }

    /// The change as a percentage of the open.
    pub fn percent(&self) -> f64 {
        (self.price - self.open) / self.open * 100.0
    }
}

/// The quotes case's market: its rows and its random numbers, mulberry32
/// from seed 42, as every program's.
pub struct Quotes {
    pub rows: Vec<Quote>,
    seed: u32,
}

impl Default for Quotes {
    fn default() -> Self {
        Self::new()
    }
}

impl Quotes {
    pub fn new() -> Self {
        let mut q = Quotes { rows: Vec::with_capacity(QUOTE_ROWS), seed: 42 };
        for _ in 0..QUOTE_ROWS {
            let mut symbol = [0u8; 4];
            for b in &mut symbol {
                *b = b'A' + (q.next() * 26.0) as u8;
            }
            let px = 20.0 + q.next() * 480.0;
            let volume = (q.next() * 1_000_000.0) as i64;
            q.rows.push(Quote { symbol, open: px, price: px, volume });
        }
        q
    }

    fn next(&mut self) -> f64 {
        self.seed = self.seed.wrapping_add(0x6d2b79f5);
        let mut t = self.seed;
        t = (t ^ (t >> 15)).wrapping_mul(t | 1);
        t ^= t.wrapping_add((t ^ (t >> 7)).wrapping_mul(t | 61));
        (t ^ (t >> 14)) as f64 / 4294967296.0
    }

    /// Changes a quarter of the rows, drawn with replacement, and returns
    /// the rows it changed.
    pub fn tick(&mut self) -> Vec<usize> {
        let mut touched = Vec::with_capacity(QUOTE_ROWS / 4);
        for _ in 0..QUOTE_ROWS / 4 {
            let i = (self.next() * QUOTE_ROWS as f64) as usize;
            self.rows[i].price *= 1.0 + (self.next() - 0.5) * 0.004;
            self.rows[i].volume += (self.next() * 500.0) as i64;
            touched.push(i);
        }
        touched
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    // The market after 1,000 ticks, as the Go programs' package
    // (internal/bench) pins it.
    #[test]
    fn same_market_as_go() {
        let mut m = Quotes::new();
        for _ in 0..1000 {
            m.tick();
        }
        let q = m.rows[7];
        assert_eq!(format!("{} {:.6} {}", q.symbol(), q.price, q.volume), "FHTW 256.835223 264295");
    }
}
