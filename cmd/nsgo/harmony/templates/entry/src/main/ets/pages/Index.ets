import { webview } from '@kit.ArkWeb';
import { image } from '@kit.ImageKit';
import { util } from '@kit.ArkTS';
import { i18n, resourceManager } from '@kit.LocalizationKit';
import { deviceInfo, pasteboard } from '@kit.BasicServicesKit';
import { photoAccessHelper } from '@kit.MediaLibraryKit';
import { camera, cameraPicker } from '@kit.CameraKit';
import { fileIo as fs, fileUri, picker } from '@kit.CoreFileKit';
import { accessibility } from '@kit.AccessibilityKit';
import { formProvider, formBindingData } from '@kit.FormKit';
import { display, window, typeNode, BuilderNode, NodeController, FrameNode, UIContext, componentSnapshot } from '@kit.ArkUI';
import { displaySync } from '@kit.ArkGraphics2D';
import { common, ConfigurationConstant } from '@kit.AbilityKit';
import MeasureText, { MeasureOptions } from '@ohos.measure';

// The host the harmonyos target builds: the app's engine runs in WebAssembly
// in a 1x1 Web view, and this page draws what it mounts with ArkUI views.
// internal/platform/harmonyos writes the operations, one JSON array a batch.

const ORIGIN: string = 'https://nsgo.local/';
const ROOT: string = 'web/';
// internal/driver's ViewKind.
const KIND_TEXT: number = 2;
const KIND_INPUT: number = 3;
const KIND_SCROLL: number = 4;
const KIND_IMAGE: number = 5;
const KIND_MODAL: number = 6;
const KIND_CANVAS: number = 7;
const KIND_WEB: number = 8;
const KIND_VIDEO: number = 9;

// DENSITY is the screen's pixels per vp, which a canvas needs to size an
// image drawn from its pixels. The device reading sets it, and density
// reads the display if a canvas draws first.
let DENSITY: number = 0;

// WidgetData is what the app hands its home-screen card: the kind it
// updated and the text its 2x2 card shows.
interface WidgetData {
  kind: string;
  title: string;
}

// FolderItem is one entry the host lists in a folder the user picked.
interface FolderItem {
  n: string;
  d: boolean;
  s: number;
}

// PickedFolderInfo is a folder the folder picker gave: its name and the
// URI the host reads its files through.
interface PickedFolderInfo {
  n: string;
  u: string;
}

// AlertSpec is a native alert the engine sends: its ID, title, message,
// whether it is a sheet, whether it may be cancelled, and its buttons with
// AlertRole numbers (0 default, 1 cancel, 2 destructive).
interface AlertButton {
  l: string;
  r: number;
}

interface AlertSpec {
  id: number;
  t: string;
  m: string;
  s: boolean;
  c: boolean;
  a: AlertButton[];
}

// PickedImage is one image the photo picker returned: its name, media type
// and bytes, base64-encoded, as the engine's PickedFile.
interface PickedImage {
  n: string;
  m: string;
  d: string;
}

// PickedFileInfo is one file the document picker returned: its name, media
// type, size and the path it was copied to in the app's cache.
interface PickedFileInfo {
  n: string;
  m: string;
  s: number;
  p: string;
}

// mimeOf guesses a file's media type from its extension.
function mimeOf(path: string): string {
  const dot = path.lastIndexOf('.');
  const ext = dot >= 0 ? path.slice(dot + 1).toLowerCase() : '';
  switch (ext) {
    case 'txt':
      return 'text/plain';
    case 'json':
      return 'application/json';
    case 'pdf':
      return 'application/pdf';
    case 'png':
      return 'image/png';
    case 'jpg':
    case 'jpeg':
      return 'image/jpeg';
    case 'zip':
      return 'application/zip';
  }
  return 'application/octet-stream';
}

// Launch is what EntryAbility stores from the Want that opened the app:
// nsgo- parameters as the engine's flags, and the link that opened it.
interface Launch {
  args: string[];
  url: string;
}

// Device is what the engine reads as its environment, in deviceJSON's
// field names. Every value is the system's, never a guess of the page's.
interface Device {
  Width: number;
  Height: number;
  SafeTop: number;
  SafeRight: number;
  SafeBottom: number;
  SafeLeft: number;
  Keyboard: number;
  PixelRatio: number;
  FontScale: number;
  Language: string;
  OS: string;
  OSMajor: number;
  OSMinor: number;
  Dark: boolean;
  Tablet: boolean;
  RTL: boolean;
  ScreenReader: boolean;
  Lifecycle: number;
  OpenedURL: string;
}

// CanvasOp is one drawing operation the engine sent: internal/driver's
// DrawOp, short keys and omitted zeros.
// TextRun is one styled run of a text with spans.
interface TextRun {
  t: string;
  c?: string;
  s?: number;
  w?: number;
  i?: boolean;
  f?: string;
}

interface CanvasOp {
  k: number;
  imgd?: string;
  x?: number;
  y?: number;
  x1?: number;
  y1?: number;
  x2?: number;
  y2?: number;
  w?: number;
  h?: number;
  r?: number;
  a?: number;
  a0?: number;
  a1?: number;
  ry?: number;
  ccw?: boolean;
  c?: string;
  cap?: number;
  join?: number;
  eo?: boolean;
  t?: string;
  ff?: string;
  fs?: number;
  fw?: number;
  fi?: boolean;
  g?: CanvasGradientData;
  co?: number;
  d?: number[];
  img?: string;
}

interface CanvasGradientData {
  radial?: boolean;
  x1?: number;
  y1?: number;
  r1?: number;
  x2?: number;
  y2?: number;
  r2?: number;
  s?: CanvasStop[];
}

interface CanvasStop {
  o: number;
  c: string;
}

// fitOf turns the engine's resize mode into ArkUI's fit.
function fitOf(fit: string): ImageFit {
  switch (fit) {
    case 'contain':
      return ImageFit.Contain;
    case 'fill':
      return ImageFit.Fill;
    case 'none':
      return ImageFit.None;
    case 'scale-down':
      return ImageFit.ScaleDown;
  }
  return ImageFit.Cover;
}

class VNode {
  id: number = 0;
  kind: number = 1;
  parent: number = 0;
  x: number = 0;
  y: number = 0;
  w: number = 0;
  h: number = 0;
  // The engine's absolute frame, for presses, and the view a press here
  // goes to: itself when it reports presses, else its nearest ancestor.
  ax: number = 0;
  ay: number = 0;
  hit: number = 0;
  bg: string = '';
  radius: number = 0;
  bw: number = 0;
  bc: string = '';
  op: number = 1;
  hidden: boolean = false;
  // needFocus and needBlur ask the renderer to move or take the field's
  // keyboard focus once, as ItemFocus and ItemBlur say.
  needFocus: boolean = false;
  needBlur: boolean = false;
  // selStart and selEnd are the selection ItemSelection asked for, -1 for
  // none: setTextSelection applies it once, after the value.
  selStart: number = -1;
  selEnd: number = -1;
  // testID, pointer and label are ItemViewProps': the id a UI test finds
  // the view by, how it takes hits, and what assistive technology says.
  testID: string = '';
  pointer: number = 0;
  label: string = '';
  accLabel: string = '';
  // A screen's system header: its title, whether it shows, and a back
  // button to the screen below. Its height and colors are the core's:
  // a large title's header is taller, and a transparent one leaves its
  // content showing.
  isScreen: boolean = false;
  scrTitle: string = '';
  scrHidden: boolean = false;
  scrCanGoBack: boolean = false;
  scrBackTitle: string = '';
  scrHeight: number = 44;
  scrTransparent: boolean = false;
  scrBg: string = '';
  scrTitleColor: string = '';
  scrTintColor: string = '';
  // anim is a modal's appearance: driver.ModalAnimation's slide, fade or
  // none. It is 1 (fade) for every other kind.
  anim: number = 1;
  text: string = '';
  fs: number = 16;
  fw: number = 400;
  italic: boolean = false;
  family: string = '';
  color: string = '#000000';
  lh: number = 0;
  align: number = 0;
  maxLines: number = 0;
  ph: string = '';
  value: string = '';
  masked: boolean = false;
  // An image's URI, its fit, and the bytes it embeds, base64.
  src: string = '';
  fit: string = 'cover';
  data: string = '';
  // A canvas's drawing, as operations to replay.
  drawing: string = '';
  // A web view's page, whether its scripts run, and the command it waits for.
  webSrc: string = '';
  noJS: boolean = false;
  webCmd: number = 0;
  webCmdValue: string = '';
  // webController is the web view's, made with the node.
  webController: webview.WebviewController | undefined = undefined;
  // A video's source, how it plays and where it is, and its controller.
  videoSrc: string = '';
  videoPaused: boolean = false;
  videoMuted: boolean = false;
  videoRepeat: boolean = false;
  videoControls: boolean = false;
  videoFit: string = 'contain';
  videoSeek: number = -1;
  videoPausedShown: boolean = false;
  videoController: VideoController | undefined = undefined;
  // Animated values folded into the view's look.
  tx: number = 0;
  ty: number = 0;
  scx: number = 1;
  scy: number = 1;
  rot: number = 0;
  // A text's styled runs, when it has spans.
  runs: TextRun[] = [];
  press: boolean = false;
  long: boolean = false;
  // A scroll view's refresh: whether it refreshes, whether it is, and how
  // far a pull must go before releasing it refreshes.
  refreshable: boolean = false;
  refreshing: boolean = false;
  refreshAt: number = 64;
  // A scroll view's content size, its offsets as the engine set them, and
  // how it scrolls.
  cw: number = 0;
  ch: number = 0;
  sx: number = 0;
  sy: number = 0;
  skey: string = '';
  // zmin, zmax and zoom are a zoomable scroll view's range and scale;
  // pinch says whether fingers zoom it, and zcx, zcy where an engine
  // zoom asked to keep.
  zmin: number = 1;
  zmax: number = 1;
  zoom: number = 1;
  pinch: boolean = true;
  zcx: number = 0;
  zcy: number = 0;
  // zset says a zoom request or a pinch set the scale, so a later
  // scroll config's starting scale does not reset it.
  zset: boolean = false;
  horizontal: boolean = false;
  hideBar: boolean = false;
  // matchFrame is where the element this view replaces was drawn, so a
  // shared-element transition starts there.
  matchFrame: MatchFrame | undefined = undefined;
  // dirty says a batch changed this view, so a frame writes its attributes
  // again: a batch that names one view must not walk the whole tree's.
  dirty: boolean = true;
  // queued says the batch already listed this view to write.
  queued: boolean = false;
  children: number[] = [];
}

// MatchFrame is a view's frame on the page.
interface MatchFrame {
  x: number;
  y: number;
  w: number;
  h: number;
}

// PinchState is a two-finger pinch in progress on a zoomable scroll
// view: which one, the fingers' first distance and the scale it began
// at.
interface PinchState {
  id: number;
  dist: number;
  scale: number;
}

// StickyCell is one cell of a scroll view that stays at its top, in the
// engine's main-axis offsets: from s it travels with the offset to e.
interface StickyCell {
  c: number;
  s: number;
  e: number;
}

// StickyShift is how far a sticky cell travels from its frame and the
// axis it travels along, kept so a batch that rewrote its position does
// not move it back.
class StickyShift {
  d: number = 0;
  across: boolean = false;
}

class Tree {
  nodes: Map<number, VNode> = new Map();
  root: number = -1;
  // sticky are the cells each scroll view pins as it scrolls.
  sticky: Map<number, StickyCell[]> = new Map();
  // The status bar: a driver.StatusBarStyle and whether it shows.
  sbStyle: number = 1;
  sbHidden: boolean = false;
  // appMode is the driver.Appearance the app shows in.
  appMode: number = 0;
  // layoutMs and layoutCurve are this batch's layout animation, which a
  // shared-element match runs over.
  layoutMs: number = 300;
  layoutCurve: number = 3;
  // gone collects the ids this batch deletes, whose FrameNodes the
  // renderer drops before it draws, since the engine reuses ids.
  gone: number[] = [];
  // zoomed collects the scroll views this batch zoomed, which the renderer
  // scales once it has drawn.
  zoomed: number[] = [];
  // structural says the batch created, moved or removed views, so the tree
  // is walked; geometry says a frame or content size changed, so the
  // absolute frames are measured again. A batch of only changed views skips
  // both, and the renderer writes just those the batch names.
  structural: boolean = false;
  geometry: boolean = false;
  dirtyIDs: number[] = [];

  // scrollAt returns the refreshable scroll view of id's subtree that
  // holds the point, the last drawn first, or found when none does.
  // scrollNodeAt returns the scroll view of id's subtree that holds the
  // point, the last drawn first, or found when none does.
  scrollNodeAt(id: number, x: number, y: number, found: number): number {
    const v = this.nodes.get(id);
    if (v === undefined || v.hidden || x < v.ax || y < v.ay || x >= v.ax + v.w || y >= v.ay + v.h) {
      return found;
    }
    let at = v.kind === KIND_SCROLL ? v.id : found;
    for (let i = v.children.length - 1; i >= 0; i--) {
      at = this.scrollNodeAt(v.children[i], x, y, at);
    }
    return at;
  }

  scrollAt(id: number, x: number, y: number, found: number): number {
    const v = this.nodes.get(id);
    if (v === undefined || v.hidden || x < v.ax || y < v.ay || x >= v.ax + v.w || y >= v.ay + v.h) {
      return found;
    }
    let at = v.kind === KIND_SCROLL && v.refreshable ? v.id : found;
    for (let i = v.children.length - 1; i >= 0; i--) {
      at = this.scrollAt(v.children[i], x, y, at);
    }
    return at;
  }

  // takeGone returns the ids the last batch deleted and clears them.
  takeGone(): number[] {
    const g = this.gone;
    this.gone = [];
    return g;
  }

  // takeZoomed returns the ids the last batch zoomed and clears them.
  takeZoomed(): number[] {
    const z = this.zoomed;
    this.zoomed = [];
    return z;
  }

  // takeDirty returns the views the last batch changed and clears them.
  takeDirty(): number[] {
    const d = this.dirtyIDs;
    this.dirtyIDs = [];
    for (let i = 0; i < d.length; i++) {
      const v = this.nodes.get(d[i]);
      if (v !== undefined) {
        v.queued = false;
      }
    }
    return d;
  }

  // takeStructural and takeGeometry report and clear what the last batch
  // changed of the tree's shape and frames.
  takeStructural(): boolean {
    const s = this.structural;
    this.structural = false;
    return s;
  }

  takeGeometry(): boolean {
    const g = this.geometry;
    this.geometry = false;
    return g;
  }

  add(ops: string): void {
    const batch = JSON.parse(ops) as Object[];
    for (let i = 0; i < batch.length; i++) {
      const o = batch[i] as Object[];
      const code = o[0] as string;
      const id = o[1] as number;
      if (code === 'c') {
        const v = new VNode();
        v.id = id;
        v.kind = o[2] as number;
        this.nodes.set(id, v);
        this.structural = true;
        this.geometry = true;
        const g = this.gone.indexOf(id);
        if (g >= 0) {
          this.gone.splice(g, 1);
        }
      } else if (code === 'root') {
        this.root = id;
        this.structural = true;
        this.geometry = true;
      } else if (code === 'app') {
        this.appMode = o[2] as number;
      } else if (code === 'sb') {
        this.sbStyle = o[2] as number;
        this.sbHidden = o[3] as boolean;
      } else if (code === 'la') {
        this.layoutMs = o[2] as number;
        this.layoutCurve = o[3] as number;
      } else if (code === 'sticky') {
        this.sticky.set(id, JSON.parse(o[2] as string) as StickyCell[]);
        this.geometry = true;
      } else if (code === 'scr') {
        const n = this.nodes.get(id);
        if (n !== undefined) {
          n.isScreen = true;
          n.scrTitle = o[2] as string;
          n.scrHidden = o[3] as boolean;
          n.scrCanGoBack = o[4] as boolean;
          n.scrBackTitle = o[5] as string;
          n.scrHeight = o[6] as number;
          n.scrTransparent = o[7] as boolean;
          n.scrBg = o[8] as string;
          n.scrTitleColor = o[9] as string;
          n.scrTintColor = o[10] as string;
        }
      } else if (code === 'i') {
        const v = this.nodes.get(id);
        if (v === undefined) {
          continue;
        }
        this.structural = true;
        this.geometry = true;
        this.detach(v);
        const parentId = o[2] as number;
        const before = o[3] as number;
        v.parent = parentId;
        const p = this.nodes.get(parentId);
        if (p === undefined) {
          continue;
        }
        // Children are replaced rather than changed in place, so the view
        // that draws them sees a new array and lays them out again.
        if (before === 0) {
          p.children = p.children.concat([id]);
        } else {
          const at = p.children.indexOf(before);
          if (at < 0) {
            p.children = p.children.concat([id]);
          } else {
            p.children = p.children.slice(0, at).concat([id], p.children.slice(at));
          }
        }
      } else if (code === 'r' || code === 'd') {
        const v = this.nodes.get(id);
        if (v === undefined) {
          continue;
        }
        this.structural = true;
        this.geometry = true;
        this.detach(v);
        if (code === 'd') {
          this.nodes.delete(id);
          this.sticky.delete(id);
          this.gone.push(id);
        }
      } else {
        const v = this.nodes.get(id);
        if (v === undefined) {
          continue;
        }
        v.dirty = true
        if (!v.queued) {
          v.queued = true;
          this.dirtyIDs.push(id);
        }
        if (code === 'f' || code === 'ss' || code === 'sc') {
          this.geometry = true;
        }
        if (code === 'match') {
          // The batch has not deleted Before yet, so its frame is here
          // for the new view to start from. Both are the page's, since
          // the two views need not share a parent.
          const before = this.nodes.get(o[2] as number);
          if (before !== undefined) {
            v.matchFrame = { x: before.ax, y: before.ay, w: before.w, h: before.h };
          }
          continue;
        }
        if (code === 'f') {
          v.x = o[2] as number;
          v.y = o[3] as number;
          v.w = o[4] as number;
          v.h = o[5] as number;
        } else if (code === 's') {
          v.bg = o[2] as string;
          v.radius = o[3] as number;
          v.bw = o[4] as number;
          v.bc = o[5] as string;
          v.op = o[6] as number;
          v.hidden = o[7] as boolean;
          v.tx = o[8] as number;
          v.ty = o[9] as number;
          v.scx = o[10] as number;
          v.scy = o[11] as number;
          v.rot = o[12] as number;
        } else if (code === 't') {
          v.text = o[2] as string;
          v.fs = o[3] as number;
          v.fw = o[4] as number;
          v.italic = o[5] as boolean;
          v.family = o[6] as string;
          v.color = o[7] as string;
          v.lh = o[8] as number;
          v.align = o[9] as number;
          v.maxLines = o[10] as number;
        } else if (code === 'e') {
          v.press = o[2] as boolean;
          v.long = o[3] as boolean;
        } else if (code === 'in') {
          v.ph = o[2] as string;
          v.fs = o[3] as number;
          v.fw = o[4] as number;
          v.color = o[5] as string;
          v.masked = o[7] as boolean;
        } else if (code === 'iv') {
          v.value = o[2] as string;
        } else if (code === 'im') {
          v.src = o[2] as string;
          v.fit = o[3] as string;
          v.data = o[4] as string;
        } else if (code === 'cv') {
          v.drawing = o[2] as string;
        } else if (code === 'tr') {
          v.runs = JSON.parse(o[2] as string) as TextRun[];
        } else if (code === 'sc') {
          v.horizontal = o[2] as boolean;
          v.hideBar = o[3] as boolean;
          v.refreshable = o[4] as boolean;
          v.refreshing = o[5] as boolean;
          v.refreshAt = o[6] as number;
          v.zmin = o[7] as number;
          v.zmax = o[8] as number;
          if (!v.zset) {
            v.zoom = o[9] as number;
          }
          v.pinch = o[10] as boolean;
        } else if (code === 'zoom') {
          v.zoom = o[2] as number;
          v.zcx = o[3] as number;
          v.zcy = o[4] as number;
          v.zset = true;
          this.zoomed.push(id);
        } else if (code === 'ss') {
          v.cw = o[2] as number;
          v.ch = o[3] as number;
        } else if (code === 'web') {
          v.webSrc = o[2] as string;
          v.noJS = o[3] as boolean;
        } else if (code === 'wcmd') {
          v.webCmd = o[2] as number;
          v.webCmdValue = o[3] as string;
        } else if (code === 'video') {
          v.videoSrc = o[2] as string;
          v.videoPaused = o[3] as boolean;
          v.videoMuted = o[4] as boolean;
          v.videoRepeat = o[5] as boolean;
          v.videoControls = o[6] as boolean;
          v.videoFit = o[7] as string;
        } else if (code === 'vseek') {
          v.videoSeek = o[2] as number;
        } else if (code === 'al') {
          v.accLabel = o[2] as string;
        } else if (code === 'vp') {
          v.testID = o[2] as string;
          v.pointer = o[3] as number;
          v.label = o[4] as string;
        } else if (code === 'sel') {
          v.selStart = o[2] as number;
          v.selEnd = o[3] as number;
        } else if (code === 'fo') {
          v.needFocus = true;
        } else if (code === 'bl') {
          v.needBlur = true;
        } else if (code === 'mo') {
          v.anim = o[2] as number;
        } else if (code === 'so') {
          v.sx = o[2] as number;
          v.sy = o[3] as number;
          v.skey = v.sx + ',' + v.sy;
        }
      }
    }
  }

  detach(v: VNode): void {
    if (v.parent === 0) {
      return;
    }
    const p = this.nodes.get(v.parent);
    if (p !== undefined) {
      p.children = p.children.filter((c: number) => c !== v.id);
    }
    v.parent = 0;
  }

  // measure walks the tree from its root, giving each view its frame in the
  // window and the view a press on it goes to.
  measure(): void {
    if (this.root >= 0) {
      this.walk(this.root, 0, 0, 0);
    }
  }

  walk(id: number, ox: number, oy: number, hit: number): void {
    const v = this.nodes.get(id);
    if (v === undefined) {
      return;
    }
    v.ax = ox + v.x;
    v.ay = oy + v.y;
    v.hit = v.press ? v.id : hit;
    for (let i = 0; i < v.children.length; i++) {
      this.walk(v.children[i], v.ax, v.ay, v.hit);
    }
  }

  at(id: number): VNode | undefined {
    return this.nodes.get(id);
  }

  top(): VNode | undefined {
    return this.root < 0 ? undefined : this.nodes.get(this.root);
  }
}

// Bridge is what the module calls in this page, as nsgoHarmony.
class Bridge {
  host: Index | undefined = undefined;

  ready(): void {
    console.info('NSGO harmony: engine ready');
    // The module is up, so what was measured before it started can be sent.
    if (this.host !== undefined) {
      this.host.readEnv();
    }
  }

  submit(ops: string): void {
    if (this.host !== undefined) {
      this.host.apply(ops);
    }
  }

  tick(): void {
    if (this.host !== undefined) {
      this.host.tick();
    }
  }

  idle(): void {
    if (this.host !== undefined) {
      this.host.idle();
    }
  }

  perf(text: string): void {
    if (this.host !== undefined) {
      this.host.setPerf(text);
    }
  }

  snapshot(count: number): void {
    if (this.host !== undefined) {
      this.host.snapshot(count);
    }
  }

  scrollBy(id: number, dx: number, dy: number): void {
    const sc = this.host?.renderer.scrollers.get(id);
    if (sc !== undefined) {
      sc.scrollBy(dx, dy);
    }
  }

  device(): string {
    const host = this.host;
    if (host === undefined) {
      return '{}';
    }
    return host.deviceJSON();
  }

  // launch answers the page before the module starts, so a Want's nsgo-
  // flags reach go.argv without a second build for each run.
  launch(): string {
    return AppStorage.get<string>('nsgoLaunch') ?? '{"args":[],"url":""}';
  }

  alert(json: string): void {
    if (this.host !== undefined) {
      this.host.alert(json);
    }
  }

  clipread(kind: number, id: number): void {
    if (this.host !== undefined) {
      this.host.clipread(kind, id);
    }
  }

  clipset(text: string): void {
    if (this.host !== undefined) {
      this.host.clipset(text);
    }
  }

  clipimage(data: string): void {
    if (this.host !== undefined) {
      this.host.clipimage(data);
    }
  }

  dismisskeyboard(): void {
    if (this.host !== undefined) {
      this.host.dismissKeyboard();
    }
  }

  announce(text: string): void {
    if (this.host !== undefined) {
      this.host.announce(text);
    }
  }

  title(label: string): void {
    if (this.host !== undefined) {
      this.host.title(label);
    }
  }

  updatewidget(data: string): void {
    if (this.host !== undefined) {
      this.host.updatewidget(data);
    }
  }

  listdir(uri: string, name: string): string {
    return this.host === undefined ? '[]' : this.host.listdir(uri, name);
  }

  readfile(uri: string, name: string): string {
    return this.host === undefined ? '' : this.host.readfile(uri, name);
  }

  pickdirectories(id: number, limit: number): void {
    if (this.host !== undefined) {
      this.host.pickdirectories(id, limit);
    }
  }

  imageinfo(id: number, uri: string, data: string): void {
    if (this.host !== undefined) {
      this.host.imageinfo(id, uri, data);
    }
  }

  openurl(id: number, url: string): void {
    if (this.host !== undefined) {
      this.host.openurl(id, url);
    }
  }

  pickimages(id: number, limit: number): void {
    if (this.host !== undefined) {
      this.host.pickimages(id, limit);
    }
  }

  pickfiles(id: number, limit: number): void {
    if (this.host !== undefined) {
      this.host.pickfiles(id, limit);
    }
  }

  takephoto(id: number): void {
    if (this.host !== undefined) {
      this.host.takephoto(id);
    }
  }

  savefile(id: number, name: string, mime: string, data: string): void {
    if (this.host !== undefined) {
      this.host.savefile(id, name, mime, data);
    }
  }

  openfile(id: number, path: string, mime: string): void {
    if (this.host !== undefined) {
      this.host.openfile(id, path, mime);
    }
  }

  // backunhandled says the engine left the back gesture to the system.
  backunhandled(): void {
    if (this.host !== undefined) {
      this.host.finish();
    }
  }

  measure(text: string, size: number, weight: number, italic: boolean, family: string, width: number): string {
    const opts = {
      textContent: text,
      fontSize: size,
      fontWeight: weight,
      fontStyle: italic ? FontStyle.Italic : FontStyle.Normal
    } as MeasureOptions;
    if (width > 0) {
      const wrapped = {
        textContent: text,
        fontSize: size,
        fontWeight: weight,
        fontStyle: italic ? FontStyle.Italic : FontStyle.Normal,
        constraintWidth: width
      } as MeasureOptions;
      const sw = MeasureText.measureTextSize(wrapped);
      return '[' + px2vp(Number(sw.width)) + ',' + px2vp(Number(sw.height)) + ']';
    }
    const s = MeasureText.measureTextSize(opts);
    return '[' + px2vp(Number(s.width)) + ',' + px2vp(Number(s.height)) + ']';
  }
}

@Entry
@Component
struct Index {
  controller: webview.WebviewController = new webview.WebviewController();
  bridge: Bridge = new Bridge();
  tree: Tree = new Tree();
  // The renderer mounts the engine's tree on FrameNodes; its controller is
  // the NodeContainer the page shows.
  renderer: Renderer = new Renderer();
  renderController: RenderController = new RenderController();
  private res: resourceManager.ResourceManager | undefined = undefined;
  private hostCtx: common.UIAbilityContext | undefined = undefined;
  private win: window.Window | undefined = undefined;
  private safeTop: number = 0;
  private safeBottom: number = 0;
  private keyboard: number = 0;
  // The display clock that runs while something animates.
  private sync: displaySync.DisplaySync | undefined = undefined;
  // pinchState is the pinch in progress over a zoomable scroll view.
  private pinchState: PinchState | undefined = undefined;
  private ticking: boolean = false;
  // appMode is the appearance the app last asked for, -1 before any.
  // systemDark is the system's, and dark is the one the status bar's
  // automatic style follows: the app's when it forces one.
  private appMode: number = -1;
  private systemDark: boolean = false;
  private dark: boolean = false;
  // The performance monitor's readings, shown when --nsgo-fps asks.
  @State private perfText: string = '';
  // picking is set while a system picker shows: its back closes it rather
  // than the app's back gesture, which would leave the app.
  private picking: boolean = false;
  // A new Want, a system configuration change or a lifecycle move bumps
  // this; the watcher asks the engine to read its environment again.
  @StorageLink('nsgoLaunchVer') @Watch('onLaunchChange') launchVer: number = 0;

  onLaunchChange(): void {
    this.readEnv();
  }


  // clipread answers ItemReadClipboard: the clipboard's text, or whether it
  // holds text at all without reading it.
  clipread(kind: number, id: number): void {
    const pb = pasteboard.getSystemPasteboard();
    if (kind === 1) {
      let has = false;
      try {
        has = pb.hasDataSync();
      } catch (e) {
        console.error('NSGO harmony: clipboard: ' + e);
      }
      this.send('clip', id, 1, 0, has ? '1' : '0');
      return;
    }
    if (kind !== 0) {
      this.send('clip', id, 0, 0, '');
      return;
    }
    pb.getData().then((data: pasteboard.PasteData) => {
      let text = '';
      try {
        text = data.getPrimaryText();
      } catch (e) {
        text = '';
      }
      this.send('clip', id, 0, 0, text);
    }).catch(() => {
      this.send('clip', id, 0, 0, '');
    });
  }

  // clipset replaces the clipboard with text.
  clipset(text: string): void {
    const data = pasteboard.createData(pasteboard.MIMETYPE_TEXT_PLAIN, text);
    pasteboard.getSystemPasteboard().setData(data).catch((e: Error) => {
      console.error('NSGO harmony: clipboard: ' + e);
    });
  }

  // clipimage puts an image on the system clipboard, decoding the app's
  // bytes into a PixelMap first.
  clipimage(data: string): void {
    try {
      const bytes = new util.Base64Helper().decodeSync(data);
      const source = image.createImageSource(bytes.buffer);
      source.createPixelMap().then((pm: image.PixelMap) => {
        const paste = pasteboard.createData(pasteboard.MIMETYPE_PIXELMAP, pm);
        pasteboard.getSystemPasteboard().setData(paste).then(() => {
          source.release();
        }).catch((e: Error) => {
          console.error('NSGO harmony: clipboard image: ' + e);
          source.release();
        });
      }).catch((e: Error) => {
        console.error('NSGO harmony: clipboard image: ' + e);
      });
    } catch (e) {
      console.error('NSGO harmony: clipboard image: ' + e);
    }
  }

  // openurl hands a URL to the system, which opens it in whichever app
  // takes it, and answers ItemOpenURL with whether one did.
  openurl(id: number, url: string): void {
    const ctx = this.hostCtx;
    if (ctx === undefined) {
      this.send('openurl', id, 0, 0, '');
      return;
    }
    ctx.startAbility({
      action: 'ohos.want.action.viewData',
      uri: url,
      entities: ['entity.system.browsable']
    }).then(() => {
      this.send('openurl', id, 1, 0, '');
    }).catch((e: Error) => {
      console.error('NSGO harmony: open URL: ' + e);
      this.send('openurl', id, 0, 0, '');
    });
  }
  setPerf(text: string): void {
    this.perfText = text;
  }

  // pinchTouch zooms the zoomable scroll view under a pinch's middle and
  // reports whether it took the event. A pinch ends with the fingers.
  private pinchTouch(e: TouchEvent): boolean {
    if (e.touches.length !== 2) {
      this.pinchState = undefined;
      return false;
    }
    const a = e.touches[0];
    const b = e.touches[1];
    const dx = a.x - b.x;
    const dy = a.y - b.y;
    const dist = Math.sqrt(dx * dx + dy * dy);
    const mx = (a.x + b.x) / 2;
    const my = (a.y + b.y) / 2;
    const state = this.pinchState;
    if (state === undefined) {
      const id = this.tree.scrollNodeAt(this.tree.root, mx, my, 0);
      const v = this.tree.nodes.get(id);
      if (v === undefined || !v.pinch || v.zmax <= v.zmin) {
        return false;
      }
      this.pinchState = { id: id, dist: dist, scale: v.zoom };
      return true;
    }
    const v = this.tree.nodes.get(state.id);
    if (v === undefined) {
      this.pinchState = undefined;
      return false;
    }
    if (state.dist > 0) {
      const z = Math.min(Math.max(state.scale * dist / state.dist, v.zmin), v.zmax);
      this.renderer.zoomTo(v, z, mx, my);
    }
    return true;
  }

  // snapshot takes the view's picture for a test: the root's pixels as a
  // PNG, base64, answered on the page's own global.
  snapshot(count: number): void {
    componentSnapshot.get('nsgo-root').then((px: image.PixelMap) => {
      const packer = image.createImagePacker();
      packer.packToData(px, { format: 'image/png', quality: 100 }).then((buf: ArrayBuffer) => {
        const b64 = new util.Base64Helper().encodeToStringSync(new Uint8Array(buf));
        this.controller.runJavaScript('nsgoSnapshotReply(' + count + ',"' + b64 + '")');
        packer.release();
        px.release();
      }).catch((e: Error) => {
        console.error('NSGO harmony: snapshot pack failed: ' + e.message);
        this.controller.runJavaScript('nsgoSnapshotReply(' + count + ',"")');
      });
    }).catch((e: Error) => {
      console.error('NSGO harmony: snapshot failed: ' + e.message);
      this.controller.runJavaScript('nsgoSnapshotReply(' + count + ',"")');
    });
  }

  // alert shows the engine's alert as the system's dialog, as the other
  // platforms do; its buttons answer with EventAlert carrying the alert's
  // ID and the button's index, or -1 when the user cancels it.
  alert(json: string): void {
    const a = JSON.parse(json) as AlertSpec;
    const buttons: AlertDialogButtonOptions[] = [];
    for (let i = 0; i < a.a.length; i++) {
      const at = i;
      if (a.a[i].r === 2) {
        buttons.push({ value: a.a[i].l, fontColor: '#d32f2f', action: () => { this.answer(a.id, at); } });
      } else {
        buttons.push({ value: a.a[i].l, action: () => { this.answer(a.id, at); } });
      }
    }
    if (a.c) {
      AlertDialog.show({ title: a.t, message: a.m, buttons: buttons, autoCancel: true,
        cancel: () => { this.answer(a.id, -1); } });
    } else {
      AlertDialog.show({ title: a.t, message: a.m, buttons: buttons, autoCancel: false });
    }
  }

  answer(id: number, index: number): void {
    this.send('alert', id, index, 0, '');
  }

  // pickimages shows the system's photo picker and answers ItemPickImages
  // with the bytes of what the user picked, base64-encoded.
  pickimages(id: number, limit: number): void {
    this.picking = true;
    const picker = new photoAccessHelper.PhotoViewPicker();
    picker.select({
      MIMEType: photoAccessHelper.PhotoViewMIMETypes.IMAGE_TYPE,
      maxSelectNumber: limit > 0 ? limit : 1
    }).then((result: photoAccessHelper.PhotoSelectResult) => {
      const files: PickedImage[] = [];
      const uris = result.photoUris;
      for (let i = 0; i < uris.length; i++) {
        try {
          const f = fs.openSync(uris[i], fs.OpenMode.READ_ONLY);
          const size = fs.statSync(f.fd).size;
          const buf = new ArrayBuffer(size);
          fs.readSync(f.fd, buf);
          fs.closeSync(f);
          const data = new util.Base64Helper().encodeToStringSync(new Uint8Array(buf));
          files.push({ n: 'image-' + i + '.jpg', m: 'image/jpeg', d: data });
        } catch (e) {
          console.error('NSGO harmony: pick image: ' + e);
        }
      }
      this.picking = false;
      this.send('picked', id, 0, 0, JSON.stringify(files));
    }).catch((e: Error) => {
      console.error('NSGO harmony: picker: ' + e);
      this.picking = false;
      this.send('picked', id, 0, 0, '[]');
    });
  }

  // pickfiles shows the system's document picker and copies what the user
  // picked into the app's cache, answering ItemPickFiles with the paths.
  pickfiles(id: number, limit: number): void {
    this.picking = true;
    const ctx = this.hostCtx;
    if (ctx === undefined) {
      this.picking = false;
      this.send('pickedfiles', id, 0, 0, '[]');
      return;
    }
    const p = new picker.DocumentViewPicker();
    p.select({ maxSelectNumber: limit > 0 ? limit : 1 }).then((uris: Array<string>) => {
      const files: PickedFileInfo[] = [];
      for (let i = 0; i < uris.length; i++) {
        const uri = uris[i];
        try {
          const src = fs.openSync(uri, fs.OpenMode.READ_ONLY);
          const size = fs.statSync(src.fd).size;
          const dest = ctx.cacheDir + '/nsgo-pick-' + Date.now() + '-' + i;
          fs.copyFileSync(src.fd, dest);
          fs.closeSync(src);
          files.push({ n: 'picked-' + i, m: mimeOf(dest), s: size, p: dest });
        } catch (e) {
          console.error('NSGO harmony: pick file: ' + e);
        }
      }
      this.picking = false;
      this.send('pickedfiles', id, 0, 0, JSON.stringify(files));
    }).catch((e: Error) => {
      console.error('NSGO harmony: picker: ' + e);
      this.picking = false;
      this.send('pickedfiles', id, 0, 0, '[]');
    });
  }

  // takephoto shows the system's camera and answers ItemTakePhoto with the
  // photo it took, as PickImages answers its images.
  takephoto(id: number): void {
    this.picking = true;
    const ctx = this.hostCtx;
    if (ctx === undefined) {
      this.picking = false;
      this.send('picked', id, 0, 0, '[]');
      return;
    }
    cameraPicker.pick(ctx, [cameraPicker.PickerMediaType.PHOTO], {
      cameraPosition: camera.CameraPosition.CAMERA_POSITION_BACK
    }).then((result: cameraPicker.PickerResult) => {
      const files: PickedImage[] = [];
      if (result.resultUri !== '') {
        try {
          const f = fs.openSync(result.resultUri, fs.OpenMode.READ_ONLY);
          const size = fs.statSync(f.fd).size;
          const buf = new ArrayBuffer(size);
          fs.readSync(f.fd, buf);
          fs.closeSync(f);
          files.push({ n: 'photo.jpg', m: 'image/jpeg', d: new util.Base64Helper().encodeToStringSync(new Uint8Array(buf)) });
        } catch (e) {
          console.error('NSGO harmony: take photo: ' + e);
        }
      }
      this.picking = false;
      this.send('picked', id, 0, 0, JSON.stringify(files));
    }).catch((e: Error) => {
      console.error('NSGO harmony: camera: ' + e);
      this.picking = false;
      this.send('picked', id, 0, 0, '[]');
    });
  }

  // savefile shows the system's save dialog and writes the bytes there.
  savefile(id: number, name: string, mime: string, data: string): void {
    this.picking = true;
    const p = new picker.DocumentViewPicker();
    p.save({ newFileNames: [name] }).then((uris: Array<string>) => {
      if (uris.length === 0) {
        this.picking = false;
        this.send('saved', id, 0, 0, '');
        return;
      }
      try {
        const bytes = new util.Base64Helper().decodeSync(data);
        const f = fs.openSync(uris[0], fs.OpenMode.READ_WRITE | fs.OpenMode.CREATE | fs.OpenMode.TRUNC);
        fs.writeSync(f.fd, bytes.buffer);
        fs.closeSync(f);
        this.picking = false;
        this.send('saved', id, 1, 0, '');
      } catch (e) {
        this.picking = false;
        this.send('saved', id, 0, 0, '' + e);
      }
    }).catch((e: Error) => {
      this.picking = false;
      this.send('saved', id, 0, 0, e.message);
    });
  }

  // openfile hands a file to the app the system associates with it.
  openfile(id: number, path: string, mime: string): void {
    const ctx = this.hostCtx;
    if (ctx === undefined) {
      this.send('opened', id, 0, 0, '');
      return;
    }
    ctx.startAbility({
      action: 'ohos.want.action.viewData',
      uri: fileUri.getUriFromPath(path),
      type: mime
    }).then(() => {
      this.send('opened', id, 1, 0, '');
    }).catch((e: Error) => {
      console.error('NSGO harmony: open file: ' + e);
      this.send('opened', id, 0, 0, e.message);
    });
  }

  // dismissKeyboard takes keyboard focus away, which closes the keyboard.
  dismissKeyboard(): void {
    this.getUIContext().getFocusController().clearFocus();
  }

  // imageinfo loads an image ahead of showing it and answers ItemImageInfo
  // with its size in pixels, or why it could not.
  imageinfo(id: number, uri: string, data: string): void {
    let source: image.ImageSource | undefined = undefined;
    try {
      if (data !== '') {
        const bytes = new util.Base64Helper().decodeSync(data);
        source = image.createImageSource(bytes.buffer);
      } else {
        source = image.createImageSource(uri);
      }
    } catch (e) {
      this.send('imageinfo', id, 0, 0, '' + e);
      return;
    }
    const s = source;
    if (s === undefined) {
      this.send('imageinfo', id, 0, 0, 'no image');
      return;
    }
    s.getImageInfo().then((info: image.ImageInfo) => {
      this.send('imageinfo', id, info.size.width, info.size.height, '');
      s.release();
    }).catch((e: Error) => {
      this.send('imageinfo', id, 0, 0, e.message);
      s.release();
    });
  }

  // title names the app in the task switcher, which has no window title.
  title(label: string): void {
    const ctx = this.hostCtx;
    if (ctx === undefined) {
      return;
    }
    ctx.setMissionLabel(label).catch((e: Error) => {
      console.error('NSGO harmony: title: ' + e);
    });
  }

  // updatewidget writes what the app last gave its home-screen card into
  // the app's files, which the card's provider reads, and refreshes any
  // card already on the home screen. A card has no Web view and runs in
  // another process, so the data goes through the file they share.
  updatewidget(data: string): void {
    console.info('NSGO harmony: updatewidget ' + data);
    const ctx = this.hostCtx;
    if (ctx === undefined) {
      return;
    }
    const dir = ctx.getApplicationContext().filesDir;
    const path = dir + '/nsgo-widget-data.json';
    try {
      const f = fs.openSync(path, fs.OpenMode.CREATE | fs.OpenMode.TRUNC | fs.OpenMode.WRITE_ONLY);
      fs.writeSync(f.fd, data);
      fs.closeSync(f);
    } catch (e) {
      console.error('NSGO harmony: widget data: ' + e);
    }
    let ids: string[] = [];
    try {
      const f = fs.openSync(dir + '/nsgo-form-ids.json', fs.OpenMode.READ_ONLY);
      const stat = fs.statSync(f.fd);
      const buf = new ArrayBuffer(stat.size);
      fs.readSync(f.fd, buf);
      fs.closeSync(f);
      ids = JSON.parse(new util.TextDecoder().decodeToString(new Uint8Array(buf))) as string[];
    } catch (e) {
      // No card has been added yet, so there is nothing to refresh.
      return;
    }
    try {
      const cfg = JSON.parse(data) as WidgetData;
      const binding = formBindingData.createFormBindingData({ title: cfg.title });
      for (let i = 0; i < ids.length; i++) {
        formProvider.updateForm(ids[i], binding).catch((e: Error) => {
          console.error('NSGO harmony: update card: ' + e);
        });
      }
    } catch (e) {
      console.error('NSGO harmony: refresh cards: ' + e);
    }
  }

  // listdir lists one folder of a picked directory. The Go side calls it
  // synchronously as the app walks the folder as an fs.FS.
  listdir(uri: string, name: string): string {
    try {
      const base = name === '.' ? uri : uri + '/' + name;
      const names = fs.listFileSync(base);
      const out: FolderItem[] = [];
      for (let i = 0; i < names.length; i++) {
        try {
          const st = fs.statSync(base + '/' + names[i]);
          out.push({ n: names[i], d: st.isDirectory(), s: st.size });
        } catch (e) {
          // A vanished entry is simply left out of the listing.
        }
      }
      return JSON.stringify(out);
    } catch (e) {
      console.error('NSGO harmony: list folder: ' + e);
      return '[]';
    }
  }

  // readfile gives the Go side a picked folder's file as base64.
  readfile(uri: string, name: string): string {
    try {
      const f = fs.openSync(uri + '/' + name, fs.OpenMode.READ_ONLY);
      const st = fs.statSync(f.fd);
      const buf = new ArrayBuffer(st.size);
      fs.readSync(f.fd, buf);
      fs.closeSync(f);
      return new util.Base64Helper().encodeToStringSync(new Uint8Array(buf));
    } catch (e) {
      console.error('NSGO harmony: read folder file: ' + e);
      return '';
    }
  }

  // pickdirectories shows the system's folder picker. It reports the
  // folders as name/URI pairs; the Go side reads their contents through
  // listdir and readfile as the app asks for them.
  pickdirectories(id: number, limit: number): void {
    this.picking = true;
    const p = new picker.DocumentViewPicker();
    p.select({ selectMode: picker.DocumentSelectMode.FOLDER,
      maxSelectNumber: limit > 0 ? limit : 1 }).then((uris: Array<string>) => {
      const out: PickedFolderInfo[] = [];
      for (let i = 0; i < uris.length; i++) {
        const u = uris[i];
        out.push({ n: u.substring(u.lastIndexOf('/') + 1), u: u });
      }
      this.picking = false;
      this.send('pickeddirectories', id, 0, 0, JSON.stringify(out));
    }).catch((e: Error) => {
      console.error('NSGO harmony: folder picker: ' + e);
      this.picking = false;
      this.send('pickeddirectories', id, 0, 0, '[]');
    });
  }

  // announce has the screen reader say a message, where one runs.
  announce(text: string): void {
    const ctx = this.hostCtx;
    if (ctx === undefined) {
      return;
    }
    try {
      const info = new accessibility.EventInfo('announceForAccessibility', ctx.applicationInfo.name, 'focus');
      info.textAnnouncedForAccessibility = text;
      accessibility.sendAccessibilityEvent(info).catch((e: Error) => {
        console.error('NSGO harmony: announce: ' + e);
      });
    } catch (e) {
      console.error('NSGO harmony: announce: ' + e);
    }
  }

  // finish leaves the app when the engine has nothing left to take the
  // back gesture, as a back press does on the other targets.
  finish(): void {
    const ctx = this.hostCtx;
    if (ctx !== undefined) {
      ctx.terminateSelf().catch(() => {});
    }
  }

  // onBackPress sends the system's back gesture to the engine first: it
  // closes the topmost modal, pops a screen or lets an app's handler take
  // it, and answers with ItemBackUnhandled when nothing does.
  onBackPress(): boolean {
    if (this.picking) {
      this.picking = false;
      return true;
    }
    if (this.tree.root >= 0) {
      this.send('back', 0, 0, 0, '');
    }
    return true;
  }

  aboutToAppear(): void {
    const host = this.getUIContext().getHostContext() as common.UIAbilityContext;
    if (host) {
      this.hostCtx = host;
      this.res = host.resourceManager;
    }
    this.bridge.host = this;
    this.renderer.host = this;
    nsgoHost = this;
    this.renderController.renderer = this.renderer;
    this.watchWindow();
  }

  // watchWindow measures what the system bars cover, and measures again when
  // they or the window change, telling the engine through nsgoEnv.
  async watchWindow(): Promise<void> {
    const ctx = this.hostCtx;
    if (ctx === undefined) {
      return;
    }
    try {
      const w: window.Window = await window.getLastWindow(ctx);
      this.win = w;
      this.readEnv();
      w.on('avoidAreaChange', () => {
        this.readEnv();
      });
      w.on('windowSizeChange', () => {
        this.readEnv();
      });
    } catch (e) {
      console.error('NSGO harmony: window failed: ' + e);
    }
  }

  readEnv(): void {
    if (this.win !== undefined) {
      try {
        const bars = this.win.getWindowAvoidArea(window.AvoidAreaType.TYPE_SYSTEM);
        this.safeTop = px2vp(bars.topRect.height);
        this.renderer.safeTop = this.safeTop;
        this.safeBottom = px2vp(bars.bottomRect.height);
        const kb = this.win.getWindowAvoidArea(window.AvoidAreaType.TYPE_KEYBOARD);
        this.keyboard = px2vp(kb.bottomRect.height);
      } catch (e) {
        console.error('NSGO harmony: avoid area failed: ' + e);
      }
    }
    try {
      this.controller.runJavaScript('nsgoEnv()');
    } catch (e) {
      console.error('NSGO harmony: env report failed: ' + e);
    }
    this.applyStatusBar();
  }

  deviceJSON(): string {
    const d = display.getDefaultDisplaySync();
    const config = this.hostCtx?.config;
    const locale = i18n.System.getSystemLocale().replace('_', '-');
    let screenReader = false;
    try {
      screenReader = accessibility.isScreenReaderOpenSync();
    } catch (e) {
      console.error('NSGO harmony: screen reader failed: ' + e);
    }
    let launch: Launch = { args: [], url: '' };
    try {
      launch = JSON.parse(AppStorage.get<string>('nsgoLaunch') ?? '{"args":[],"url":""}') as Launch;
    } catch (e) {
      console.error('NSGO harmony: launch failed: ' + e);
    }
    DENSITY = d.densityPixels;
    const dark = config?.colorMode === ConfigurationConstant.ColorMode.COLOR_MODE_DARK;
    this.systemDark = dark;
    this.dark = this.appMode === 2 ? true : (this.appMode === 1 ? false : dark);
    const out: Device = {
      Width: px2vp(d.width), Height: px2vp(d.height),
      SafeTop: this.safeTop, SafeRight: 0, SafeBottom: this.safeBottom, SafeLeft: 0,
      Keyboard: this.keyboard,
      PixelRatio: d.densityPixels, FontScale: config?.fontSizeScale ?? 1,
      Language: locale, OS: 'ohos',
      OSMajor: deviceInfo.majorVersion, OSMinor: deviceInfo.seniorVersion,
      Dark: dark,
      Tablet: deviceInfo.deviceType === 'tablet',
      RTL: i18n.isRTL(locale), ScreenReader: screenReader,
      Lifecycle: AppStorage.get<number>('nsgoLifecycle') ?? 0,
      OpenedURL: launch.url ?? ''
    };
    return JSON.stringify(out);
  }

  apply(ops: string): void {
    this.renderer.apply(ops);
    this.applyStatusBar();
    this.applyAppearance();
  }

  // touch reports the fingers on the surface, which the engine hit-tests
  // against its frames and hands to the views that listen for them. A
  // finger that lifted may be in the changed ones alone, so both lists
  // make up the event.
  touch(e: TouchEvent): void {
    if (this.pinchTouch(e)) {
      return;
    }
    let phase = 2;
    if (e.type === TouchType.Down) {
      phase = 1;
    } else if (e.type === TouchType.Up) {
      phase = 3;
    } else if (e.type === TouchType.Cancel) {
      phase = 4;
    }
    this.renderer.pull(e, phase);
    const moved = new Map<number, boolean>();
    const changed = e.changedTouches;
    for (let i = 0; i < changed.length; i++) {
      moved.set(changed[i].id, true);
    }
    const parts: string[] = [];
    const down = e.touches;
    for (let i = 0; i < down.length; i++) {
      const t = down[i];
      parts.push('{"i":' + t.id + ',"x":' + t.x + ',"y":' + t.y +
        ',"c":' + (moved.get(t.id) === true ? 'true' : 'false') + '}');
    }
    for (let i = 0; i < changed.length; i++) {
      const t = changed[i];
      let seen = false;
      for (let k = 0; k < down.length; k++) {
        if (down[k].id === t.id) {
          seen = true;
          break;
        }
      }
      if (!seen) {
        parts.push('{"i":' + t.id + ',"x":' + t.x + ',"y":' + t.y + ',"c":true}');
      }
    }
    this.controller.runJavaScript(`nsgoTouches(${phase},${Date.now()},'[` + parts.join(',') + `]')`);
  }

  // applyAppearance shows the app in the appearance it asks for, which the
  // system's configuration then carries back to the engine with the rest of
  // the environment.
  applyAppearance(): void {
    const mode = this.tree.appMode;
    if (mode === this.appMode) {
      return;
    }
    this.appMode = mode;
    const ctx = this.hostCtx;
    if (ctx === undefined) {
      return;
    }
    const m = mode === 1 ? ConfigurationConstant.ColorMode.COLOR_MODE_LIGHT :
      (mode === 2 ? ConfigurationConstant.ColorMode.COLOR_MODE_DARK :
        ConfigurationConstant.ColorMode.COLOR_MODE_NOT_SET);
    try {
      ctx.setColorMode(m);
    } catch (e) {
      console.error('NSGO harmony: color mode failed: ' + e);
    }
    // The bar's automatic style follows the appearance at once, and the
    // configuration the change brings follows with the rest of it.
    this.dark = mode === 2 ? true : (mode === 1 ? false : this.systemDark);
    this.applyStatusBar();
    setTimeout(() => { this.readEnv(); }, 0);
  }

  // applyStatusBar gives the window the status bar the engine asked for.
  applyStatusBar(): void {
    const w = this.win;
    if (w === undefined) {
      return;
    }
    try {
      w.setWindowSystemBarEnable(this.tree.sbHidden ? [] : ['status']);
      // Auto follows the appearance, as the engine's StatusBarAuto says.
      const light = this.tree.sbStyle === 3 || (this.tree.sbStyle === 1 && this.dark);
      w.setWindowSystemBarProperties({
        statusBarColor: light ? '#000000' : '#ffffff',
        statusBarContentColor: light ? '#ffffff' : '#000000'
      });
    } catch (e) {
      console.error('NSGO harmony: status bar: ' + e);
    }
  }

  frame(): void {
    this.controller.runJavaScript('nsgoFrame()');
  }

  // tick keeps the display clock running: each frame asks the engine for
  // one, and idle stops it once nothing animates.
  tick(): void {
    if (this.sync === undefined) {
      const sync = displaySync.create();
      sync.setExpectedFrameRateRange({ min: 30, max: 120, expected: 60 });
      sync.on('frame', () => {
        this.frame();
      });
      this.sync = sync;
    }
    if (!this.ticking) {
      this.ticking = true;
      const sync = this.sync;
      if (sync !== undefined) {
        sync.start();
      }
    }
  }

  idle(): void {
    if (this.ticking) {
      this.ticking = false;
      const sync = this.sync;
      if (sync !== undefined) {
        sync.stop();
      }
    }
  }

  // scrolled reports where a scroll view is, as the user left it.
  scrolled(id: number, x: number, y: number): void {
    this.send('scroll', id, x, y, '');
  }

  // zoomed reports a scroll view's scale and the offset a pinch left it
  // at.
  zoomed(id: number, z: number, x: number, y: number): void {
    this.controller.runJavaScript('nsgoEvent("zoom",' + id + ',' + z + ',' + x + ',' + y + ')');
  }

  send(name: string, id: number, x: number, y: number, text: string): void {
    let js = 'nsgoEvent("' + name + '",' + id + ',' + x + ',' + y;
    if (text !== '') {
      js += ',"' + text.replace(/\\/g, '\\\\').replace(/"/g, '\\"') + '"';
    }
    js += ')';
    this.controller.runJavaScript(js);
  }

  serve(url: string): WebResourceResponse {
    let rel = ROOT + 'index.html';
    const i = url.indexOf(ROOT);
    if (i >= 0) {
      rel = url.substring(i);
      const q = rel.indexOf('?');
      if (q >= 0) {
        rel = rel.substring(0, q);
      }
    }
    let type = 'text/html';
    if (rel.endsWith('.wasm')) {
      type = 'application/wasm';
    } else if (rel.endsWith('.js')) {
      type = 'application/javascript';
    } else if (rel.endsWith('.json')) {
      type = 'application/json';
    }
    const resp = new WebResourceResponse();
    resp.setResponseCode(200);
    resp.setResponseHeader([
      { headerKey: 'Content-Type', headerValue: type },
      { headerKey: 'Access-Control-Allow-Origin', headerValue: '*' }
    ]);
    if (this.res) {
      try {
        resp.setResponseData(this.res.getRawFileContentSync(rel).buffer);
      } catch (e) {
        // A file the page asks for may be absent, such as a test's or the
        // monitor's: the page reads a 404 as none.
        resp.setResponseCode(404);
        resp.setResponseHeader([{ headerKey: 'Content-Type', headerValue: 'text/plain' }]);
      }
    }
    return resp;
  }

  build() {
    Stack({ alignContent: Alignment.TopStart }) {
      NodeContainer(this.renderController)
        .id('nsgo-root')

      Web({ src: ORIGIN + ROOT + 'index.html', controller: this.controller })
        .javaScriptAccess(true)
        .domStorageAccess(true)
        .javaScriptProxy({
          object: this.bridge,
          name: 'nsgoHarmony',
          methodList: ['ready', 'submit', 'tick', 'idle', 'device', 'measure', 'perf', 'snapshot', 'scrollBy', 'launch', 'alert', 'backunhandled', 'clipread', 'clipset', 'clipimage', 'dismisskeyboard', 'announce', 'title', 'updatewidget', 'listdir', 'readfile', 'pickdirectories', 'imageinfo', 'openurl', 'pickimages', 'pickfiles', 'savefile', 'openfile', 'takephoto'],
          controller: this.controller
        })
        .onInterceptRequest((event) => {
          return this.serve(event.request.getRequestUrl());
        })
          .onConsole((event) => {
            console.info('NSGO harmony page: ' + event.message.getMessage());
            return false;
          })
        .width(1)
        .height(1)
        .position({ x: 0, y: 0 })

      if (this.perfText !== '') {
        Text(this.perfText)
          .fontSize(10)
          .fontColor('#ffffff')
          .backgroundColor('rgba(0,0,0,0.65)')
          .padding(6)
          .borderRadius(4)
          .position({ right: 8, top: 44 })
      }
    }
    .width('100%')
    .height('100%')
    .backgroundColor('#000000')
    .focusable(true)
    .onKeyEvent((e: KeyEvent) => {
      // The engine's keymap is on the Go side, so the page sends the raw
      // key and its modifiers: driver.FindKey matches it there.
      let mods = 0;
      if (e.getModifierKeyState !== undefined) {
        if (e.getModifierKeyState(['Ctrl'])) {
          mods |= 1;
        }
        if (e.getModifierKeyState(['Alt'])) {
          mods |= 2;
        }
        if (e.getModifierKeyState(['Shift'])) {
          mods |= 4;
        }
      }
      if (e.metaKey === 1) {
        mods |= 8;
      }
      this.send('key', e.keyCode, e.type === KeyType.Up ? 1 : 0, mods, e.keyText);
      return false;
    })
  }
}

// NodeImageView draws an image the engine mounted, from the bytes it embeds
// (base64, decoded into a PixelMap), from the app's resources ("asset:",
// read out of the HAP's rawfile directory) or from the URI the app gave.
@Component
struct NodeImageView {
  @Prop @Watch('reload') src: string = '';
  @Prop @Watch('reload') fit: string = 'cover';
  @Prop @Watch('reload') data: string = '';
  @State pixels: image.PixelMap | undefined = undefined;

  aboutToAppear(): void {
    this.decode();
  }

  reload(): void {
    this.decode();
  }

  async decode(): Promise<void> {
    if (this.data !== '') {
      try {
        const bytes = new util.Base64Helper().decodeSync(this.data);
        const source = image.createImageSource(bytes.buffer as ArrayBuffer);
        this.pixels = await source.createPixelMap();
        source.release();
      } catch (e) {
        console.error('NSGO harmony: image failed: ' + e);
      }
      return;
    }
    if (!this.src.startsWith('asset:')) {
      return;
    }
    // The build puts the app's resources beside the page, and the HAP
    // carries them in its rawfile directory, which only the host reads.
    const rel = this.src.substring('asset:'.length).replace(/^\/+/, '');
    try {
      const ctx = getContext(this) as common.UIAbilityContext;
      const bytes = ctx.resourceManager.getRawFileContentSync('web/resources/' + rel);
      const source = image.createImageSource(bytes.buffer as ArrayBuffer);
      this.pixels = await source.createPixelMap();
      source.release();
    } catch (e) {
      console.error('NSGO harmony: image ' + this.src + ' failed: ' + e);
    }
  }

  build() {
    if (this.pixels !== undefined) {
      Image(this.pixels)
        .objectFit(fitOf(this.fit))
        .width('100%')
        .height('100%')
    } else if (this.src !== '' && !this.src.startsWith('asset:')) {
      Image(this.src)
        .objectFit(fitOf(this.fit))
        .width('100%')
        .height('100%')
        .onError((e: ImageError) => {
          console.error('NSGO harmony: image ' + this.src + ' failed: ' +
            e.message + ' ' + e.componentWidth + 'x' + e.componentHeight);
        })
    }
  }
}


// CanvasView replays a canvas's drawing into ArkUI's Canvas. The engine
// sends the whole drawing again when it changes, and the canvas redraws.
@Component
struct CanvasView {
  @Prop @Watch('paint') drawing: string = '';
  private ctx: CanvasRenderingContext2D = new CanvasRenderingContext2D(new RenderingContextSettings(true));
  private ready: boolean = false;
  // images are the pictures drawings draw, decoded once; a repaint runs
  // when one arrives.
  private images: Map<string, image.PixelMap> = new Map();
  private loading: Set<string> = new Set();
  private missing: Set<string> = new Set();

  aboutToAppear(): void {
  }

  // roundRect adds a rounded rectangle to the current path.
  private roundRect(x: number, y: number, w: number, h: number, r: number): void {
    const ctx = this.ctx;
    const rr = Math.min(r, Math.min(w, h) / 2);
    ctx.moveTo(x + rr, y);
    ctx.lineTo(x + w - rr, y);
    ctx.quadraticCurveTo(x + w, y, x + w, y + rr);
    ctx.lineTo(x + w, y + h - rr);
    ctx.quadraticCurveTo(x + w, y + h, x + w - rr, y + h);
    ctx.lineTo(x + rr, y + h);
    ctx.quadraticCurveTo(x, y + h, x, y + h - rr);
    ctx.lineTo(x, y + rr);
    ctx.quadraticCurveTo(x, y, x + rr, y);
    ctx.closePath();
  }

  // drawImage draws an image op: at its natural size when W and H are 0,
  // else into that rectangle, from the part X1, Y1, X2, Y2 of it when the
  // source rectangle is given. The op is in points and the pixels are not.
  private drawImage(ctx: CanvasRenderingContext2D, pm: image.PixelMap, o: CanvasOp): void {
    const d = density();
    const size = pm.getImageInfoSync().size;
    const nw = size.width / d;
    const nh = size.height / d;
    const w = o.w ?? 0;
    const h = o.h ?? 0;
    const dw = w === 0 && h === 0 ? nw : w;
    const dh = w === 0 && h === 0 ? nh : h;
    const x = o.x ?? 0;
    const y = o.y ?? 0;
    if ((o.x2 ?? 0) === 0 && (o.y2 ?? 0) === 0) {
      ctx.drawImage(pm, x, y, dw, dh);
      return;
    }
    ctx.drawImage(pm, (o.x1 ?? 0) * d, (o.y1 ?? 0) * d, (o.x2 ?? 0) * d, (o.y2 ?? 0) * d, x, y, dw, dh);
  }

  // loadImage decodes an image op once; a repaint draws it when it comes.
  private loadImage(key: string, uri: string, data: string): void {
    if (this.loading.has(key) || this.missing.has(key)) {
      return;
    }
    if (data === '' && !uri.startsWith('asset:')) {
      this.missing.add(key);
      console.warn('NSGO harmony: canvas image ' + uri + ' is not drawn: a URL is not loaded here');
      return;
    }
    this.loading.add(key);
    this.decodeImage(key, uri, data);
  }

  private async decodeImage(key: string, uri: string, data: string): Promise<void> {
    try {
      let bytes: Uint8Array = new Uint8Array(0);
      if (data !== '') {
        bytes = new util.Base64Helper().decodeSync(data);
      } else {
        const rel = uri.substring('asset:'.length).replace(/^\/+/, '');
        const ctx = getContext(this) as common.UIAbilityContext;
        bytes = ctx.resourceManager.getRawFileContentSync('web/resources/' + rel);
      }
      const source = image.createImageSource(bytes.buffer as ArrayBuffer);
      this.images.set(key, await source.createPixelMap());
      source.release();
      this.loading.delete(key);
      this.paint();
    } catch (e) {
      this.loading.delete(key);
      this.missing.add(key);
      console.error('NSGO harmony: canvas image failed: ' + e);
    }
  }

  // gradientOf builds a canvas gradient, or undefined when it is conic.
  private gradientOf(g: CanvasGradientData): CanvasGradient | undefined {
    const ctx = this.ctx;
    let grad: CanvasGradient;
    if (g.radial === true) {
      grad = ctx.createRadialGradient(g.x1 ?? 0, g.y1 ?? 0, g.r1 ?? 0, g.x2 ?? 0, g.y2 ?? 0, g.r2 ?? 0);
    } else {
      grad = ctx.createLinearGradient(g.x1 ?? 0, g.y1 ?? 0, g.x2 ?? 0, g.y2 ?? 0);
    }
    const stops = g.s ?? [];
    for (let i = 0; i < stops.length; i++) {
      grad.addColorStop(stops[i].o, stops[i].c);
    }
    return grad;
  }

  paint(): void {
    if (!this.ready || this.drawing === '') {
      return;
    }
    const ctx = this.ctx;
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.globalAlpha = 1;
    ctx.lineWidth = 1;
    ctx.lineCap = 'butt';
    ctx.lineJoin = 'miter';
    ctx.miterLimit = 10;
    ctx.setLineDash([]);
    ctx.lineDashOffset = 0;
    ctx.shadowColor = 'rgba(0,0,0,0)';
    ctx.shadowBlur = 0;
    ctx.shadowOffsetX = 0;
    ctx.shadowOffsetY = 0;
    ctx.globalCompositeOperation = 'source-over';
    ctx.clearRect(0, 0, 100000, 100000);
    let ops: CanvasOp[] = [];
    try {
      ops = JSON.parse(this.drawing) as CanvasOp[];
    } catch (e) {
      console.error('NSGO harmony: canvas failed: ' + e);
      return;
    }
    const caps: CanvasLineCap[] = ['butt', 'round', 'square'];
    const joins: CanvasLineJoin[] = ['miter', 'round', 'bevel'];
    const comps: string[] = ['source-over', 'source-in', 'source-out', 'source-atop',
      'destination-over', 'destination-in', 'destination-out', 'destination-atop',
      'lighter', 'copy', 'xor', 'multiply', 'screen', 'overlay', 'darken', 'lighten',
      'color-dodge', 'color-burn', 'hard-light', 'soft-light'];
    let fill: string | CanvasGradient = '#000000';
    let stroke: string | CanvasGradient = '#000000';
    for (let i = 0; i < ops.length; i++) {
      const o = ops[i];
      switch (o.k) {
        case 1:
          ctx.save();
          break;
        case 2:
          ctx.restore();
          break;
        case 3:
          ctx.translate(o.x ?? 0, o.y ?? 0);
          break;
        case 4:
          ctx.scale(o.x ?? 1, o.y ?? 1);
          break;
        case 5:
          ctx.rotate(o.a ?? 0);
          break;
        case 6:
          ctx.beginPath();
          break;
        case 7:
          ctx.moveTo(o.x ?? 0, o.y ?? 0);
          break;
        case 8:
          ctx.lineTo(o.x ?? 0, o.y ?? 0);
          break;
        case 9:
          ctx.quadraticCurveTo(o.x1 ?? 0, o.y1 ?? 0, o.x ?? 0, o.y ?? 0);
          break;
        case 10:
          ctx.bezierCurveTo(o.x1 ?? 0, o.y1 ?? 0, o.x2 ?? 0, o.y2 ?? 0, o.x ?? 0, o.y ?? 0);
          break;
        case 11:
          ctx.arc(o.x ?? 0, o.y ?? 0, o.r ?? 0, o.a0 ?? 0, o.a1 ?? 0, o.ccw === true);
          break;
        case 12:
          ctx.rect(o.x ?? 0, o.y ?? 0, o.w ?? 0, o.h ?? 0);
          break;
        case 13:
          this.roundRect(o.x ?? 0, o.y ?? 0, o.w ?? 0, o.h ?? 0, o.r ?? 0);
          break;
        case 14:
          ctx.ellipse(o.x ?? 0, o.y ?? 0, o.r ?? 0, o.ry ?? 0, 0, 0, Math.PI * 2);
          break;
        case 15:
          ctx.closePath();
          break;
        case 16:
          ctx.fill(o.eo === true ? 'evenodd' : 'nonzero');
          break;
        case 17:
          ctx.stroke();
          break;
        case 18:
          ctx.clip(o.eo === true ? 'evenodd' : 'nonzero');
          break;
        case 19:
          fill = o.c ?? '#000000';
          ctx.fillStyle = fill;
          break;
        case 20:
          stroke = o.c ?? '#000000';
          ctx.strokeStyle = stroke;
          break;
        case 21:
          ctx.lineWidth = o.w ?? 1;
          break;
        case 22:
          ctx.lineCap = caps[(o.cap ?? 1) - 1] ?? 'butt';
          break;
        case 23:
          ctx.lineJoin = joins[(o.join ?? 1) - 1] ?? 'miter';
          break;
        case 24:
          ctx.globalAlpha = o.a ?? 1;
          break;
        case 25:
        case 36:
          ctx.font = (o.fi === true ? 'italic ' : '') + (o.fw ?? 400) + ' ' +
            (o.fs ?? 16) + 'px ' + (o.ff !== undefined && o.ff !== '' ? o.ff : 'sans-serif');
          if (o.k === 25) {
            ctx.fillStyle = fill;
            ctx.fillText(o.t ?? '', (o.x ?? 0) + (o.x1 ?? 0), (o.y ?? 0) + (o.y1 ?? 0));
          } else {
            ctx.strokeStyle = stroke;
            ctx.strokeText(o.t ?? '', (o.x ?? 0) + (o.x1 ?? 0), (o.y ?? 0) + (o.y1 ?? 0));
          }
          break;
        case 26:
          ctx.clearRect(o.x ?? 0, o.y ?? 0, o.w ?? 0, o.h ?? 0);
          break;
        case 27:
          if (o.g !== undefined) {
            const g = this.gradientOf(o.g);
            if (g !== undefined) {
              fill = g;
              ctx.fillStyle = g;
            }
          }
          break;
        case 28:
          if (o.g !== undefined) {
            const g = this.gradientOf(o.g);
            if (g !== undefined) {
              stroke = g;
              ctx.strokeStyle = g;
            }
          }
          break;
        case 29:
          ctx.transform(o.x1 ?? 1, o.y1 ?? 0, o.x2 ?? 0, o.y2 ?? 1, o.x ?? 0, o.y ?? 0);
          break;
        case 30:
          ctx.setTransform(o.x1 ?? 1, o.y1 ?? 0, o.x2 ?? 0, o.y2 ?? 1, o.x ?? 0, o.y ?? 0);
          break;
        case 31:
          ctx.globalCompositeOperation = comps[(o.co ?? 1) - 1];
          break;
        case 32:
          ctx.setLineDash(o.d ?? []);
          break;
        case 33:
          ctx.lineDashOffset = o.w ?? 0;
          break;
        case 34:
          ctx.miterLimit = o.w ?? 10;
          break;
        case 35:
          ctx.shadowColor = o.c ?? 'rgba(0,0,0,0)';
          ctx.shadowOffsetX = o.x ?? 0;
          ctx.shadowOffsetY = o.y ?? 0;
          ctx.shadowBlur = o.r ?? 0;
          break;
        case 37: {
          const uri = o.img ?? '';
          const data = o.imgd ?? '';
          const key = data !== '' ? data : uri;
          if (key === '') {
            break;
          }
          const pm = this.images.get(key);
          if (pm === undefined) {
            this.loadImage(key, uri, data);
            break;
          }
          this.drawImage(ctx, pm, o);
          break;
        }
      }
    }
  }

  build() {
    Canvas(this.ctx)
      .width('100%')
      .height('100%')
      .onReady(() => {
        this.ready = true;
        this.paint();
      })
  }
}


// nsgoHost is the page a WebView reports to, set as it appears.
let nsgoHost: Index | undefined = undefined;


// VideoView draws a video's frame and controls, and reports where it is.
@Component
struct VideoView {
  v: VNode = new VNode();
  @State private duration: number = 0;

  build() {
    Video({ src: this.v.videoSrc, controller: this.v.videoController })
      .muted(this.v.videoMuted)
      .controls(this.v.videoControls)
      .loop(this.v.videoRepeat)
      .autoPlay(!this.v.videoPaused)
      .objectFit(fitOf(this.v.videoFit))
      .width('100%')
      .height('100%')
      .onPrepared((e: PreparedInfo) => {
        this.duration = e.duration;
        const host = nsgoHost;
        if (host !== undefined) {
          host.send('videoload', this.v.id, e.duration, 0, '');
        }
      })
      .onUpdate((e: PlaybackInfo) => {
        const host = nsgoHost;
        if (host !== undefined) {
          host.send('videoprogress', this.v.id, e.time, this.duration, '');
        }
      })
      .onFinish(() => {
        const host = nsgoHost;
        if (host !== undefined) {
          host.send('videoend', this.v.id, 0, 0, '');
        }
      })
      .onError(() => {
        const host = nsgoHost;
        if (host !== undefined) {
          host.send('videoerror', this.v.id, 0, 0, 'playback failed');
        }
      })
  }
}
// WebView draws a web view's page: the engine gives it a URL and its
// commands, and the page reports loading back.
@Component
struct WebView {
  v: VNode = new VNode();
  c: webview.WebviewController | undefined = undefined;
  host: Index | undefined = undefined;

  build() {
    if (this.c !== undefined) {
      Web({ src: this.v.webSrc, controller: this.c })
        .javaScriptAccess(!this.v.noJS)
        .onPageBegin(() => {
          const host = this.host;
          if (host !== undefined) {
            host.send('webloadstart', this.v.id, 0, 0, this.v.webSrc);
          }
        })
        .onPageEnd(() => {
          const host = this.host;
          if (host !== undefined) {
            host.send('webload', this.v.id, 0, 0, this.v.webSrc);
          }
        })
        .onErrorReceive((e: OnErrorReceiveEvent) => {
          const host = this.host;
          if (host !== undefined) {
            host.send('weberror', this.v.id, 0, 0, e.request.getRequestUrl());
          }
        })
    }
  }
}

// The builders for the leaves typeNode cannot make: rich text (Span), an
// image drawn from its bytes, and a canvas.
@Builder function spanBuilder(v: VNode) {
  Text() {
    ForEach(v.runs, (r: TextRun) => {
      Span(r.t)
        .fontSize(r.s ?? v.fs)
        .fontWeight(r.w ?? v.fw)
        .fontColor(r.c ?? v.color)
        .fontStyle(r.i === true ? FontStyle.Italic : FontStyle.Normal)
    }, (r: TextRun, i: number) => i.toString())
  }
  .textAlign(v.align === 1 ? TextAlign.Center :
    (v.align === 2 ? TextAlign.End : TextAlign.Start))
  .maxLines(v.maxLines > 0 ? v.maxLines : 100)
  .textOverflow({ overflow: TextOverflow.Ellipsis })
  .width('100%')
}

@Builder function imageBuilder(v: VNode) {
  NodeImageView({ src: v.src, fit: v.fit, data: v.data })
}

@Builder function canvasBuilder(v: VNode) {
  CanvasView({ drawing: v.drawing })
}

@Builder function webBuilder(v: VNode) {
  WebView({ v: v, c: v.webController, host: nsgoHost })
}

// HeaderView draws a screen's system header: the bar with its title, and a
// back button to the screen below. The header covers the bar and the part
// of the status bar the screen reaches under, so its background fills that
// whole area and the bar sits at its bottom.
@Component
struct HeaderView {
  v: VNode = new VNode();

  build() {
    Stack({ alignContent: Alignment.TopStart }) {
      Stack({ alignContent: Alignment.Center }) {
        Text(this.v.scrTitle)
          .fontSize(17)
          .fontWeight(FontWeight.Medium)
          .fontColor(this.v.scrTitleColor === '' ? '#000000' : this.v.scrTitleColor)
          .maxLines(1)
          .textOverflow({ overflow: TextOverflow.Ellipsis })
          .width('60%')
          .textAlign(TextAlign.Center)
        if (this.v.scrCanGoBack) {
          Row() {
            Text('‹').fontSize(28).fontColor(this.v.scrTintColor === '' ? '#007dff' : this.v.scrTintColor)
            Text(this.v.scrBackTitle)
              .fontSize(17)
              .fontColor(this.v.scrTintColor === '' ? '#007dff' : this.v.scrTintColor)
          }
          .position({ x: 8, y: 0 })
          .height('100%')
          .onClick(() => {
            const host = nsgoHost;
            if (host !== undefined) {
              host.send('back', this.v.id, 0, 0, '');
            }
          })
        }
      }
      .width('100%')
      .height(44)
      .position({ x: 0, y: this.v.scrHeight > 44 ? this.v.scrHeight - 44 : 0 })
    }
    .width('100%')
    .height(this.v.scrHeight > 0 ? this.v.scrHeight : 44)
    .backgroundColor(this.v.scrTransparent ? Color.Transparent :
      (this.v.scrBg === '' ? '#f7f7f7' : this.v.scrBg))
  }
}

@Builder function videoBuilder(v: VNode) {
  VideoView({ v: v })
}

@Builder function headerBuilder(v: VNode) {
  HeaderView({ v: v })
}

// density is the screen's pixels per vp.
function density(): number {
  if (DENSITY <= 0) {
    DENSITY = display.getDefaultDisplaySync().densityPixels;
  }
  return DENSITY;
}

// layoutCurve maps a driver.LayoutCurve to ArkUI's, whose imperative
// animation takes one of its curves rather than the driver's spring or
// bezier.
function layoutCurve(c: number): Curve {
  if (c === 0) {
    return Curve.Linear;
  }
  if (c === 1) {
    return Curve.EaseIn;
  }
  if (c === 2) {
    return Curve.EaseOut;
  }
  if (c === 4) {
    return Curve.Friction;
  }
  return Curve.EaseInOut;
}

// Renderer mounts the engine's views on FrameNodes, one node per view: a
// container is a Stack, a scroll is a Scroll around a content Stack, and a
// text, input or image is the ArkUI component of its kind. A batch changes
// the nodes it names, so no statement reads a batch version any more.
class Renderer {
  host: Index | undefined = undefined;
  ui: UIContext | undefined = undefined;
  root: FrameNode | null = null;
  nodes: Map<number, FrameNode> = new Map();
  contents: Map<number, FrameNode> = new Map();
  scrollers: Map<number, Scroller> = new Map();
  leaves: Map<number, BuilderNode<[VNode]>> = new Map();
  // webControllers are the web views' controllers, which commands use.
  webControllers: Map<number, webview.WebviewController> = new Map();
  webShown: Map<number, string> = new Map();
  videoShown: Map<number, string> = new Map();
  // headers are the system headers the screens draw, and their builders.
  headers: Map<number, FrameNode> = new Map();
  headerBuilders: Map<number, BuilderNode<[VNode]>> = new Map();
  // controllers are the text fields' TextInputControllers, which select.
  controllers: Map<number, TextInputController> = new Map();
  // shown is the text or value a node was last given, so a batch that did
  // not change it does not reset a field's caret.
  shown: Map<number, string> = new Map();
  // scrollStates are the ScrollState each scroll view was last in, so its
  // transitions say when a drag began, glided and came to rest.
  scrollStates: Map<number, number> = new Map();
  // pullId is the refreshable scroll view a finger is pulling down, and
  // pullFrom where it started.
  pullId: number = 0;
  pullFrom: number = 0;
  // cellShifts are the sticky cells' travel from their frames.
  cellShifts: Map<number, StickyShift> = new Map();
  // safeTop is the status bar's height; the tree is drawn below it.
  safeTop: number = 0;
  // applied is the scroll offset a scroll node was last given.
  applied: Map<number, string> = new Map();
  // zoomShown is the scale a zoomable scroll view's content was last
  // given.
  zoomShown: Map<number, number> = new Map();

  attach(ui: UIContext, root: FrameNode): void {
    this.ui = ui;
    this.root = root;
    this.sync();
  }

  // apply puts a batch into the tree and draws it. A frame that fails keeps
  // what it drew last and says so, since the next batch is the engine's
  // chance to set it right.
  apply(ops: string): void {
    const host = this.host;
    if (host === undefined) {
      return;
    }
    try {
      host.tree.add(ops);
      const gone = host.tree.takeGone();
      for (let i = 0; i < gone.length; i++) {
        this.forget(gone[i]);
      }
      const structural = host.tree.takeStructural();
      const geometry = host.tree.takeGeometry();
      if (geometry) {
        host.tree.measure();
      }
      if (structural) {
        host.tree.takeDirty();
        this.sync();
      } else {
        // Nothing moved: write the views the batch named, and no others.
        const dirty = host.tree.takeDirty();
        for (let i = 0; i < dirty.length; i++) {
          const v = host.tree.at(dirty[i]);
          const n = this.nodes.get(dirty[i]);
          if (v !== undefined && n !== undefined) {
            v.dirty = false;
            this.update(v, n);
          }
        }
      }
      const zoomed = host.tree.takeZoomed();
      for (let i = 0; i < zoomed.length; i++) {
        const v = host.tree.at(zoomed[i]);
        if (v !== undefined) {
          this.applyZoom(v);
        }
      }
    } catch (e) {
      console.error('NSGO harmony: apply failed: ' + JSON.stringify(e));
    }
  }

  sync(): void {
    const host = this.host;
    if (host === undefined || this.root === null || host.tree.root < 0) {
      return;
    }
    this.syncNode(host.tree.root, this.root);
  }

  // forget drops what the renderer holds for a view the batch deleted,
  // so an id the engine gives a new view next starts clean.
  forget(id: number): void {
    this.nodes.delete(id);
    this.contents.delete(id);
    this.scrollers.delete(id);
    this.leaves.delete(id);
    this.webControllers.delete(id);
    this.webShown.delete(id);
    this.videoShown.delete(id);
    this.headers.delete(id);
    this.headerBuilders.delete(id);
    this.controllers.delete(id);
    this.shown.delete(id);
    this.scrollStates.delete(id);
    this.cellShifts.delete(id);
    this.applied.delete(id);
  }

  // create makes the node for a view: its kind decides whether ArkUI's
  // component, a Stack, or a BuilderNode draws it.
  create(v: VNode): FrameNode | null {
    const ui = this.ui;
    if (ui === undefined) {
      return null;
    }
    if (v.kind === KIND_SCROLL) {
      const s = typeNode.createNode(ui, 'Scroll');
      const sc = new Scroller();
      s.initialize(sc);
      this.scrollers.set(v.id, sc);
      const content = typeNode.createNode(ui, 'Stack');
      const ca = typeNode.getAttribute(content, 'Stack');
      if (ca !== undefined) {
        ca.alignContent(Alignment.TopStart);
      }
      content.commonAttribute.width(v.cw > 0 ? v.cw : v.w).height(v.ch > 0 ? v.ch : v.h);
      s.appendChild(content);
      this.contents.set(v.id, content);
      return s;
    }
    if (v.kind === KIND_TEXT) {
      if (v.runs.length > 0) {
        return this.leaf(v, KIND_TEXT);
      }
      return typeNode.createNode(ui, 'Text');
    }
    if (v.kind === KIND_INPUT) {
      const t = typeNode.createNode(ui, 'TextInput');
      const c = new TextInputController();
      t.initialize({ controller: c });
      this.controllers.set(v.id, c);
      return t;
    }
    if (v.kind === KIND_IMAGE) {
      if (v.data !== '' || v.src.startsWith('asset:')) {
        return this.leaf(v, KIND_IMAGE);
      }
      return typeNode.createNode(ui, 'Image');
    }
    if (v.kind === KIND_CANVAS) {
      return this.leaf(v, KIND_CANVAS);
    }
    if (v.kind === KIND_WEB) {
      const c = new webview.WebviewController();
      v.webController = c;
      this.webControllers.set(v.id, c);
      return this.leaf(v, KIND_WEB);
    }
    if (v.kind === KIND_VIDEO) {
      const c = new VideoController();
      v.videoController = c;
      v.videoPausedShown = v.videoPaused;
      return this.leaf(v, KIND_VIDEO);
    }
    const s = typeNode.createNode(ui, 'Stack');
    const a = typeNode.getAttribute(s, 'Stack');
    if (a !== undefined) {
      a.alignContent(Alignment.TopStart);
    }
    return s;
  }

  leaf(v: VNode, kind: number): FrameNode | null {
    const ui = this.ui;
    if (ui === undefined) {
      return null;
    }
    const b = new BuilderNode<[VNode]>(ui);
    if (kind === KIND_TEXT) {
      b.build(wrapBuilder(spanBuilder), v);
    } else if (kind === KIND_IMAGE) {
      b.build(wrapBuilder(imageBuilder), v);
    } else if (kind === KIND_WEB) {
      b.build(wrapBuilder(webBuilder), v);
    } else if (kind === KIND_VIDEO) {
      b.build(wrapBuilder(videoBuilder), v);
    } else {
      b.build(wrapBuilder(canvasBuilder), v);
    }
    this.leaves.set(v.id, b);
    return b.getFrameNode();
  }

  syncNode(id: number, parent: FrameNode): void {
    const host = this.host;
    if (host === undefined) {
      return;
    }
    const v = host.tree.at(id);
    if (v === undefined) {
      return;
    }
    const existing = this.nodes.get(id);
    if (existing === undefined) {
      const made = this.create(v);
      if (made === null) {
        return;
      }
      this.nodes.set(id, made);
      parent.appendChild(made);
      const mf = v.matchFrame;
      if (mf !== undefined) {
        // A shared element starts at the frame of the view it replaces
        // and moves to its own over the commit's layout animation. The
        // frame is the page's, so subtract where the new parent puts it.
        made.commonAttribute.position({ x: v.x + mf.x - v.ax, y: v.y + mf.y - v.ay }).width(mf.w).height(mf.h);
        const ui = this.ui;
        if (ui !== undefined) {
          ui.animateTo({ duration: host.tree.layoutMs, curve: layoutCurve(host.tree.layoutCurve) }, () => {
            this.update(v, made);
          });
        } else {
          this.update(v, made);
        }
        v.matchFrame = undefined;
      } else {
        this.update(v, made);
      }
      return;
    }
    if (v.dirty) {
      v.dirty = false;
      this.update(v, existing);
    }
  }

  update(v: VNode, node: FrameNode): void {
    this.style(v, node);
    this.applyShift(v.id);
    if (v.kind === KIND_TEXT) {
      if (v.runs.length > 0) {
        const b = this.leaves.get(v.id);
        if (b !== undefined) {
          b.update(v);
        }
      } else {
        this.text(v, node);
      }
    } else if (v.kind === KIND_INPUT) {
      this.input(v, node);
      const ui = this.ui;
      if (v.needFocus && ui !== undefined) {
        v.needFocus = false;
        ui.getFocusController().requestFocus('nsgo-' + v.id);
      }
      if (v.needBlur && ui !== undefined) {
        v.needBlur = false;
        ui.getFocusController().clearFocus();
      }
    } else if (v.kind === KIND_IMAGE) {
      if (v.data !== '' || v.src.startsWith('asset:')) {
        const b = this.leaves.get(v.id);
        if (b !== undefined) {
          b.update(v);
        }
      } else {
        this.image(v, node);
      }
    } else if (v.kind === KIND_CANVAS) {
      const b = this.leaves.get(v.id);
      if (b !== undefined) {
        b.update(v);
      }
    } else if (v.kind === KIND_WEB) {
      const b = this.leaves.get(v.id);
      const c = this.webControllers.get(v.id);
      if (b !== undefined && c !== undefined) {
        if (this.webShown.get(v.id) !== v.webSrc) {
          this.webShown.set(v.id, v.webSrc);
          b.update(v);
        }
        const cmd = v.webCmd;
        v.webCmd = 0;
        if (cmd === 1) {
          c.refresh();
        } else if (cmd === 2) {
          c.backward();
        } else if (cmd === 3) {
          c.forward();
        } else if (cmd === 4) {
          c.stop();
        } else if (cmd === 6) {
          c.runJavaScript(v.webCmdValue);
        }
      }
    } else if (v.kind === KIND_VIDEO) {
      const b = this.leaves.get(v.id);
      const c = v.videoController;
      if (b !== undefined && c !== undefined) {
        if (this.videoShown.get(v.id) !== v.videoSrc) {
          this.videoShown.set(v.id, v.videoSrc);
          b.update(v);
        }
        if (v.videoSeek >= 0) {
          c.setCurrentTime(v.videoSeek);
          v.videoSeek = -1;
        }
        if (v.videoPaused !== v.videoPausedShown) {
          v.videoPausedShown = v.videoPaused;
          if (v.videoPaused) {
            c.pause();
          } else {
            c.start();
          }
        }
      }
    } else if (v.kind === KIND_SCROLL) {
      this.scroll(v, node);
      const content = this.contents.get(v.id);
      if (content !== undefined) {
        content.commonAttribute.width(v.cw > 0 ? v.cw : v.w).height(v.ch > 0 ? v.ch : v.h);
        this.syncChildren(v, content);
      }
    } else {
      this.screen(v, node);
      this.syncChildren(v, node);
    }
  }

  // screen draws a screen's system header over its content, and takes it
  // away when the screen hides it.
  screen(v: VNode, node: FrameNode): void {
    if (!v.isScreen) {
      return;
    }
    if (v.scrHidden) {
      const h = this.headers.get(v.id);
      if (h !== undefined) {
        node.removeChild(h);
        this.headers.delete(v.id);
        this.headerBuilders.delete(v.id);
      }
      return;
    }
    let h = this.headers.get(v.id);
    const ui = this.ui;
    if (h === undefined) {
      if (ui === undefined) {
        return;
      }
      const b = new BuilderNode<[VNode]>(ui);
      b.build(wrapBuilder(headerBuilder), v);
      this.headerBuilders.set(v.id, b);
      const f = b.getFrameNode();
      if (f === null) {
        return;
      }
      h = f;
      this.headers.set(v.id, h);
      node.appendChild(h);
    }
    const b = this.headerBuilders.get(v.id);
    if (b !== undefined) {
      b.update(v);
    }
    h.commonAttribute.width(v.w).height(v.scrHeight > 0 ? v.scrHeight : 44).position({ x: 0, y: 0 });
  }

  // style writes what a view draws on any kind of node.
  style(v: VNode, node: FrameNode): void {
    const c = node.commonAttribute;
    c.width(v.w).height(v.h).position({ x: v.x, y: v.y })
      .backgroundColor(v.bg === '' ? Color.Transparent : v.bg)
      .borderRadius(v.radius)
      .border({ width: v.bw, color: v.bc === '' ? Color.Transparent : v.bc })
      .opacity(v.op)
      .translate({ x: v.tx, y: v.ty })
      .scale({ x: v.scx, y: v.scy })
      .rotate({ angle: v.rot })
      .visibility(v.hidden ? Visibility.Hidden : Visibility.Visible);
    if (v.accLabel !== '' || v.label !== '') {
      c.accessibilityText(v.accLabel !== '' ? v.accLabel : v.label);
    }
    c.id(v.testID !== '' ? v.testID : 'nsgo-' + v.id);
    if (v.pointer === 1) {
      c.hitTestBehavior(HitTestMode.None);
    } else if (v.pointer === 2) {
      c.hitTestBehavior(HitTestMode.Transparent);
    } else if (v.pointer === 3) {
      c.hitTestBehavior(HitTestMode.BLOCK_DESCENDANTS);
    }
    if (v.press) {
      c.onClick((e: ClickEvent) => {
        const host = this.host;
        if (host !== undefined) {
          host.send('tap', v.hit, v.ax + e.x, v.ay + e.y, '');
        }
      });
    }
  }

  text(v: VNode, node: FrameNode): void {
    const t = node as typeNode.Text;
    if (this.shown.get(v.id) !== v.text) {
      t.initialize(v.text);
      this.shown.set(v.id, v.text);
    }
    const a = typeNode.getAttribute(node, 'Text');
    if (a === undefined) {
      return;
    }
    a.fontSize(v.fs).fontWeight(v.fw).fontColor(v.color)
      .textAlign(v.align === 1 ? TextAlign.Center :
        (v.align === 2 ? TextAlign.End : TextAlign.Start))
      .maxLines(v.maxLines > 0 ? v.maxLines : 100)
      .textOverflow({ overflow: TextOverflow.Ellipsis });
  }

  input(v: VNode, node: FrameNode): void {
    // The node's id is its focus key, which requestFocus names.
    node.commonAttribute.id('nsgo-' + v.id);
    const i = node as typeNode.TextInput;
    const control = this.controllers.get(v.id);
    if (this.shown.get(v.id) !== v.value) {
      if (control !== undefined) {
        i.initialize({ text: v.value, placeholder: v.ph, controller: control });
      } else {
        i.initialize({ text: v.value, placeholder: v.ph });
      }
      this.shown.set(v.id, v.value);
    }
    const a = typeNode.getAttribute(node, 'TextInput');
    if (a === undefined) {
      return;
    }
    a.fontSize(v.fs).fontColor(v.color)
      .type(v.masked ? InputType.Password : InputType.Normal)
      .lineHeight(v.h)
      .padding({ top: 0, bottom: 0 })
      .onFocus(() => {
        const host = this.host;
        if (host !== undefined) {
          host.send('focus', v.id, 0, 0, '');
        }
      })
      .onBlur(() => {
        const host = this.host;
        if (host !== undefined) {
          host.send('blur', v.id, 0, 0, '');
        }
      })
      .onChange((t: string) => {
        this.shown.set(v.id, t);
        const host = this.host;
        if (host !== undefined) {
          const n = host.tree.at(v.id);
          if (n !== undefined) {
            n.value = t;
          }
          host.send('input', v.id, 0, 0, t);
        }
      })
      .onSubmit(() => {
        const host = this.host;
        if (host !== undefined) {
          host.send('confirm', v.id, 0, 0, '');
        }
      })
      .onTextSelectionChange((start: number, end: number) => {
        const host = this.host;
        if (host !== undefined) {
          host.send('selection', v.id, start, end, '');
        }
      });
    if (v.selStart >= 0) {
      const control = this.controllers.get(v.id);
      const start = v.selStart;
      const end = v.selEnd;
      v.selStart = -1;
      v.selEnd = -1;
      // A selection needs the field's focus, which the controller has from
      // the next turn.
      const ui = this.ui;
      if (ui !== undefined) {
        ui.getFocusController().requestFocus('nsgo-' + v.id);
      }
      if (control !== undefined) {
        setTimeout(() => {
          control.setTextSelection(start, end);
        }, 0);
      }
    }
  }

  image(v: VNode, node: FrameNode): void {
    if (this.shown.get(v.id) !== v.src) {
      const im = node as typeNode.Image;
      im.initialize(v.src);
      this.shown.set(v.id, v.src);
    }
    const a = typeNode.getAttribute(node, 'Image');
    if (a !== undefined) {
      a.objectFit(fitOf(v.fit));
    }
  }

  scroll(v: VNode, node: FrameNode): void {
    const a = typeNode.getAttribute(node, 'Scroll');
    if (a !== undefined) {
      a.scrollable(v.horizontal ? ScrollDirection.Horizontal : ScrollDirection.Vertical)
        .scrollBar(v.hideBar ? BarState.Off : BarState.Auto)
        .onDidScroll((dx: number, dy: number, state: ScrollState) => {
          const sc = this.scrollers.get(v.id);
          const host = this.host;
          if (sc !== undefined && host !== undefined) {
            const o = sc.currentOffset();
            host.scrolled(v.id, o.xOffset, o.yOffset);
          }
          this.scrollPhase(v.id, state);
          this.pinSticky(v.id);
        });
    }
    const sc = this.scrollers.get(v.id);
    if (sc !== undefined && v.skey !== '' && this.applied.get(v.id) !== v.skey) {
      this.applied.set(v.id, v.skey);
      sc.scrollTo({ xOffset: v.sx, yOffset: v.sy });
    }
    this.applyZoom(v);
    this.pinSticky(v.id);
  }

  // applyZoom scales a zoomable scroll view's content about its top left,
  // grows its box so ArkUI scrolls the zoomed range, and keeps the
  // requested center where it was. The page reports the scale and offset
  // to the engine, which clamps and reports it to the app.
  applyZoom(v: VNode): void {
    const content = this.contents.get(v.id);
    if (content === undefined) {
      return;
    }
    const z = Math.min(Math.max(v.zoom, v.zmin), v.zmax);
    v.zoom = z;
    const prev = this.zoomShown.get(v.id);
    if (prev === z) {
      return;
    }
    const cw = v.cw > 0 ? v.cw : v.w;
    const ch = v.ch > 0 ? v.ch : v.h;
    content.commonAttribute.width(cw * z).height(ch * z).scale({ x: z, y: z, centerX: 0, centerY: 0 });
    let ox = 0;
    let oy = 0;
    const sc = this.scrollers.get(v.id);
    if (sc !== undefined) {
      const cur = sc.currentOffset();
      const k = prev === undefined || prev === 0 ? 1 : z / prev;
      ox = Math.max(0, (cur.xOffset + v.zcx) * k - v.zcx);
      oy = Math.max(0, (cur.yOffset + v.zcy) * k - v.zcy);
      sc.scrollTo({ xOffset: ox, yOffset: oy });
    }
    this.zoomShown.set(v.id, z);
    const host = this.host;
    if (host !== undefined) {
      host.zoomed(v.id, z, ox, oy);
    }
  }

  // zoomTo sets a scroll view's scale from a pinch at the page's point at.
  zoomTo(v: VNode, z: number, at: number, ay: number): void {
    v.zcx = at;
    v.zcy = ay;
    v.zoom = z;
    v.zset = true;
    this.applyZoom(v);
  }

  // pull refreshes the scroll view a finger pulls down from its top, past
  // the distance it asks for. ArkUI's Scroll reports no offset for a pull,
  // its content springing where the finger goes, so the page judges the
  // distance from the finger itself.
  pull(e: TouchEvent, phase: number): void {
    const t = e.changedTouches.length > 0 ? e.changedTouches[0] : undefined;
    if (phase === 1) {
      this.pullId = 0;
      const host = this.host;
      if (t === undefined || host === undefined) {
        return;
      }
      const id = host.tree.scrollAt(host.tree.root, t.x, t.y, 0);
      const sc = this.scrollers.get(id);
      if (id === 0 || sc === undefined || sc.currentOffset().yOffset > 1) {
        return;
      }
      this.pullId = id;
      this.pullFrom = t.y;
      return;
    }
    if (this.pullId === 0) {
      return;
    }
    const host = this.host;
    const v = host === undefined ? undefined : host.tree.at(this.pullId);
    if (phase === 2) {
      if (host === undefined || v === undefined || t === undefined || v.refreshing) {
        return;
      }
      if (t.y - this.pullFrom >= v.refreshAt) {
        host.send('refresh', this.pullId, t.y - this.pullFrom, 0, '');
        this.pullId = 0;
      }
      return;
    }
    this.pullId = 0;
  }

  // pinSticky moves a scroll view's sticky cells for its offset. A cell
  // travels by clamp(offset - Start, 0, End - Start), as the item says.
  // Android moves the cell with setTranslationY; ArkUI's Scroll clips a
  // translated child at its laid-out frame, so here the cell's position
  // follows the scroll instead.
  pinSticky(id: number): void {
    const host = this.host;
    const sc = this.scrollers.get(id);
    if (host === undefined || sc === undefined) {
      return;
    }
    const cells = host.tree.sticky.get(id);
    const v = host.tree.at(id);
    if (cells === undefined || v === undefined) {
      return;
    }
    const o = sc.currentOffset();
    const at = v.horizontal ? o.xOffset : o.yOffset;
    for (let i = 0; i < cells.length; i++) {
      const c = cells[i];
      const d = Math.min(Math.max(at - c.s, 0), Math.max(c.e - c.s, 0));
      let sh = this.cellShifts.get(c.c);
      if (sh === undefined) {
        sh = new StickyShift();
        this.cellShifts.set(c.c, sh);
      }
      sh.d = d;
      sh.across = v.horizontal;
      this.applyShift(c.c);
    }
  }

  // applyShift writes a sticky cell's position over its style's, which a
  // fresh batch would otherwise have set back to the plain frame.
  applyShift(cellID: number): void {
    const sh = this.cellShifts.get(cellID);
    if (sh === undefined) {
      return;
    }
    const n = this.nodes.get(cellID);
    const v = this.host === undefined ? undefined : this.host.tree.at(cellID);
    if (n === undefined || v === undefined) {
      return;
    }
    // Move it in the content's coordinates: ArkUI's Scroll clips a
    // translated child at its laid-out frame, but a moved position takes
    // part in the clip. The tree is drawn below the status bar, so a
    // pinned cell starts that far down; before it pins, d is 0 and it
    // stays at its frame. Later siblings paint above it, so lift it too.
    const off = sh.d > 0 && !sh.across ? this.safeTop : 0;
    n.commonAttribute.zIndex(1).position({ x: v.x + (sh.across ? sh.d : 0),
      y: v.y + (sh.across ? 0 : sh.d) + off });
  }

  // scrollPhase tells the engine what a scroll view's state became: a
  // drag beginning or ending, the glide a lift set going, and the rest
  // that ends either.
  scrollPhase(id: number, state: number): void {
    const host = this.host;
    if (host === undefined) {
      return;
    }
    const last = this.scrollStates.get(id);
    if (last === state) {
      return;
    }
    this.scrollStates.set(id, state);
    const dragged = last === 1;
    const glided = last === 2;
    if (state === 1) {
      if (glided) {
        host.send('momentumend', id, 0, 0, '');
      }
      host.send('dragstart', id, 0, 0, '');
      return;
    }
    if (dragged) {
      host.send('dragend', id, 0, 0, '');
    }
    if (state === 2) {
      host.send('momentumstart', id, 0, 0, '');
      return;
    }
    if (state === 0) {
      if (glided) {
        host.send('momentumend', id, 0, 0, '');
      }
      if (dragged || glided) {
        host.send('scrollend', id, 0, 0, '');
      }
    }
  }

  // syncChildren makes the FrameNode's children match the view's, in order.
  syncChildren(v: VNode, parent: FrameNode): void {
    const count = parent.getChildrenCount();
    for (let i = count - 1; i >= 0; i--) {
      const child = parent.getChild(i);
      if (child !== null && !this.wanted(v, child)) {
        parent.removeChild(child);
      }
    }
    for (let k = 0; k < v.children.length; k++) {
      this.syncNode(v.children[k], parent);
      const c = this.nodes.get(v.children[k]);
      if (c === undefined) {
        continue;
      }
      if (parent.getChild(k) === c) {
        continue;
      }
      if (this.hasChild(parent, c)) {
        parent.removeChild(c);
      }
      if (k === 0) {
        parent.insertChildAfter(c, null);
      } else {
        parent.insertChildAfter(c, parent.getChild(k - 1));
      }
    }
  }

  wanted(v: VNode, child: FrameNode): boolean {
    if (this.headers.get(v.id) === child) {
      return true;
    }
    for (let i = 0; i < v.children.length; i++) {
      if (this.nodes.get(v.children[i]) === child) {
        return true;
      }
    }
    return false;
  }

  hasChild(parent: FrameNode, child: FrameNode): boolean {
    const n = parent.getChildrenCount();
    for (let i = 0; i < n; i++) {
      if (parent.getChild(i) === child) {
        return true;
      }
    }
    return false;
  }
}

// RenderController gives the page a NodeContainer root; the Renderer mounts
// the engine's tree under it.
class RenderController extends NodeController {
  renderer: Renderer | undefined = undefined;

  makeNode(uiContext: UIContext): FrameNode | null {
    const root = typeNode.createNode(uiContext, 'Stack');
    const a = typeNode.getAttribute(root, 'Stack');
    if (a !== undefined) {
      a.alignContent(Alignment.TopStart);
    }
    root.commonAttribute.width('100%').height('100%');
    const r = this.renderer;
    // Every finger is the engine's to route: it hit-tests them against its
    // frames, so a drag on a slider, a sheet or a carousel follows them.
    root.commonAttribute.onTouch((e: TouchEvent) => {
      if (r !== undefined && r.host !== undefined) {
        r.host.touch(e);
      }
    });
    if (r !== undefined) {
      r.attach(uiContext, root);
    }
    return root;
  }
}

