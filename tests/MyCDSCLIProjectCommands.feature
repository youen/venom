 Feature: CDS CLI Project commands

 Scenario: Create Project
    Given exec cds status
    And result.code ShouldEqual 0
    And  result.systemout ShouldNotContain command not found
    When exec cds project add TEST Test 
    Then result.code ShouldEqual 0

 Scenario: List Project
    Given Create Project is successful 
    When exec cds project list
    Then result.systemout ShouldContain TEST
