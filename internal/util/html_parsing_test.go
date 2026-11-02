package util

import "testing"

func TestHTMLParser(t *testing.T) {
	expected := `<div class="requiredItemsContainer" id="RequiredItems">
													<a href="https://steamcommunity.com/workshop/filedetails/?id=1559212036" target="_blank" data-subscribed="0">
								<div class="requiredItem">
									CF																	</div>
							</a>
													<a href="https://steamcommunity.com/workshop/filedetails/?id=3690289718" target="_blank" data-subscribed="0">
								<div class="requiredItem">
									AgricultureCore																	</div>
							</a>
											</div>`

	extracted := ExtractHTMLContainerByAttr(`
	<div class="panel">
					<div class="rightSectionTopTitle condensed">Required items</div>
					<div class="rightSectionMinorText">This item requires all of the following other items</div>
					<div class="requiredItemsContainer" id="RequiredItems">
													<a href="https://steamcommunity.com/workshop/filedetails/?id=1559212036" target="_blank" data-subscribed="0">
								<div class="requiredItem">
									CF																	</div>
							</a>
													<a href="https://steamcommunity.com/workshop/filedetails/?id=3690289718" target="_blank" data-subscribed="0">
								<div class="requiredItem">
									AgricultureCore																	</div>
							</a>
											</div>
				</div>`, "id", "RequiredItems")

	if extracted != expected {
		t.Fatal("incorrect html extracted")
	}
}
