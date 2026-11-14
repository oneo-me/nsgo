import { FormExtensionAbility, formBindingData, formProvider } from '@kit.FormKit';
import { Want } from '@kit.AbilityKit';
import { fileIo as fs } from '@kit.CoreFileKit';
import { util } from '@kit.ArkTS';

// WidgetConfig is what nsgo build writes to the card's widget.json: the
// title its 2x2 card shows.
interface WidgetConfig {
  title: string;
}

// WidgetData is what the app writes when it updates its card, which takes
// precedence over widget.json once the app runs.
interface WidgetData {
  kind: string;
  title: string;
}

// The card provider and the app run in different processes, so they hand
// data through files in the app's own directory. The app writes the data
// and the ids of the cards it added; the provider reads both.
const CARD_DATA = 'nsgo-widget-data.json';
const CARD_IDS = 'nsgo-form-ids.json';
const FORM_ID_KEY = 'ohos.extra.param.key.form_identity';

function readFile(path: string): string {
  try {
    const f = fs.openSync(path, fs.OpenMode.READ_ONLY);
    const stat = fs.statSync(f.fd);
    const buf = new ArrayBuffer(stat.size);
    fs.readSync(f.fd, buf);
    fs.closeSync(f);
    return new util.TextDecoder().decodeToString(new Uint8Array(buf));
  } catch (e) {
    return '';
  }
}

function writeFile(path: string, data: string): void {
  try {
    const f = fs.openSync(path, fs.OpenMode.CREATE | fs.OpenMode.TRUNC | fs.OpenMode.WRITE_ONLY);
    fs.writeSync(f.fd, data);
    fs.closeSync(f);
  } catch (e) {
    console.error('NSGO harmony: card file: ' + e);
  }
}

// EntryFormAbility adds the app's card to the home screen. The system asks
// it for the data a card shows, and the card's own page draws it with
// ArkUI, since a card has no Web view.
export default class EntryFormAbility extends FormExtensionAbility {
  private dir(): string {
    return this.context.getApplicationContext().filesDir;
  }

  // data is what the card shows: what the app wrote last, or the title
  // nsgo build put in the HAP for the first card before the app runs.
  private data(): formBindingData.FormBindingData {
    const raw = readFile(this.dir() + '/' + CARD_DATA);
    if (raw !== '') {
      try {
        const cfg = JSON.parse(raw) as WidgetData;
        if (cfg.title) {
          return formBindingData.createFormBindingData({ title: cfg.title });
        }
      } catch (e) {
        console.error('NSGO harmony: card data: ' + e);
      }
    }
    let title = 'NSGo';
    try {
      const rawCfg = this.context.resourceManager.getRawFileContentSync('web/widget.json');
      const cfg = JSON.parse(new util.TextDecoder().decodeToString(rawCfg)) as WidgetConfig;
      if (cfg.title) {
        title = cfg.title;
      }
    } catch (e) {
      console.error('NSGO harmony: widget data: ' + e);
    }
    return formBindingData.createFormBindingData({ title: title });
  }

  private addId(formId: string): void {
    const path = this.dir() + '/' + CARD_IDS;
    let ids: string[] = [];
    const raw = readFile(path);
    if (raw !== '') {
      try {
        ids = JSON.parse(raw) as string[];
      } catch (e) {
        ids = [];
      }
    }
    if (!ids.includes(formId)) {
      ids.push(formId);
      writeFile(path, JSON.stringify(ids));
    }
  }

  onAddForm(want: Want): formBindingData.FormBindingData {
    const params = want.parameters;
    if (params !== undefined) {
      const id = params[FORM_ID_KEY];
      if (typeof id === 'string') {
        this.addId(id);
        console.info('NSGO harmony: added card ' + id);
      }
    }
    return this.data();
  }

  // onUpdateForm answers the system's timed or requested refresh with the
  // latest data, which keeps the card from going stale.
  onUpdateForm(formId: string): void {
    formProvider.updateForm(formId, this.data()).catch((e: Error) => {
      console.error('NSGO harmony: update card: ' + e);
    });
  }

  onFormEvent(formId: string, message: string): void {
    console.info('NSGO harmony: card ' + formId + ': ' + message);
  }

  onRemoveForm(formId: string): void {
    const path = this.dir() + '/' + CARD_IDS;
    const raw = readFile(path);
    if (raw === '') {
      return;
    }
    try {
      const ids = (JSON.parse(raw) as string[]).filter((id: string) => id !== formId);
      writeFile(path, JSON.stringify(ids));
    } catch (e) {
      console.error('NSGO harmony: remove card: ' + e);
    }
  }
}
