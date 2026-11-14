// WidgetCard is the app's 2x2 home-screen card. A card runs in its own
// ArkTS environment, which has no Web view, so it draws with ArkUI and reads
// the data EntryFormAbility gives it.
@Entry
@Component
struct WidgetCard {
  @LocalStorageProp('title') title: string = 'NSGo';

  build() {
    Column() {
      Text(this.title)
        .fontSize(20)
        .fontWeight(FontWeight.Medium)
        .fontColor('#000000')
    }
    .width('100%')
    .height('100%')
    .justifyContent(FlexAlign.Center)
    .backgroundColor('#ffffff')
  }
}
