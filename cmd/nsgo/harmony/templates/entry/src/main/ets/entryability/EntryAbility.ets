import { UIAbility, Want, AbilityConstant, EnvironmentCallback,
  Configuration } from '@kit.AbilityKit';
import { window } from '@kit.ArkUI';

// Launch is what a Want gives the module: the engine's flags as nsgo-
// parameters, and the link that opened the app, if any.
interface Launch {
  args: string[];
  url: string;
}

// launchOf turns a Want's nsgo- parameters into the engine's command-line
// flags: a true boolean is a flag alone (--nsgo-fps), anything else its
// value (--nsgo-lang=zh-Hans). hdc's `aa start --pb nsgo-fps true` and
// `--ps nsgo-lang de` are the equivalents at launch.
function launchOf(want: Want): string {
  const args: string[] = [];
  const params = want.parameters;
  if (params !== undefined) {
    for (const key of Object.keys(params)) {
      if (!key.startsWith('nsgo-')) {
        continue;
      }
      const value: Object = params[key];
      if (typeof value === 'boolean' && value) {
        args.push('--' + key);
      } else if (typeof value === 'string' || typeof value === 'number') {
        args.push('--' + key + '=' + value);
      }
    }
  }
  const launch: Launch = { args: args, url: want.uri ?? '' };
  return JSON.stringify(launch);
}

// ConfigWatcher answers a change of the system's appearance, font scale or
// language by asking the page to report its environment again.
class ConfigWatcher extends EnvironmentCallback {
  private ability: EntryAbility;

  constructor(ability: EntryAbility) {
    super();
    this.ability = ability;
  }

  onConfigurationUpdated(_config: Configuration): void {
    this.ability.bump();
  }

  onMemoryLevel(_level: AbilityConstant.MemoryLevel): void {
  }
}

export default class EntryAbility extends UIAbility {
  onCreate(want: Want, _launchParam: AbilityConstant.LaunchParam): void {
    AppStorage.setOrCreate('nsgoLaunch', launchOf(want));
    AppStorage.setOrCreate('nsgoLaunchVer', 0);
    AppStorage.setOrCreate('nsgoLifecycle', 0);
    this.context.getApplicationContext().on('environment', new ConfigWatcher(this));
  }

  onNewWant(want: Want, _launchParam: AbilityConstant.LaunchParam): void {
    AppStorage.setOrCreate('nsgoLaunch', launchOf(want));
    this.bump();
  }

  onForeground(): void {
    AppStorage.setOrCreate('nsgoLifecycle', 0);
    this.bump();
  }

  onBackground(): void {
    AppStorage.setOrCreate('nsgoLifecycle', 2);
    this.bump();
  }

  // bump tells the page that what a Want or the system reports changed;
  // its @StorageLink watcher sends nsgoEnv(), which the engine reads.
  bump(): void {
    const n = AppStorage.get<number>('nsgoLaunchVer') ?? 0;
    AppStorage.setOrCreate('nsgoLaunchVer', n + 1);
  }

  onWindowStageCreate(windowStage: window.WindowStage): void {
    windowStage.loadContent('pages/Index');
    // The engine lays the app out for the whole screen and insets what the
    // system bars cover, as it does on the other targets.
    try {
      windowStage.getMainWindowSync().setWindowLayoutFullScreen(true);
    } catch (e) {
      console.error('NSGO harmony: full screen failed: ' + e);
    }
  }
}
