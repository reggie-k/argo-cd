---
title: Adding UI support for searching, listing, filtering and displaying details of AppSets
authors:
  - "@reggie-k" # Authors' github accounts here.
sponsors:
  - TBD        # List all interested parties here.
reviewers:
  - "@alexmt"
  - TBD
approvers:
  - "@alexmt"
  - TBD

creation-date: 2023-05-10
last-updated: 2023-05-10
---

# Neat Enhancement Idea

This is a proposal for AppSet UI support.


## Open Questions [optional]

The React components in the UI for AppSet have much in common with the App components (side bar, tool bar, details, list, etc).
If we wanna prevent code duplication, it might be best to map what is common and create new shared components that both App and AppSet can then extend with their specifics.
However, this is a big change and will result in refactoring many (if not most) of the existing UI components.

Backend stream functionality for watching the AppSets needs to be implemented.

AppSet resource tree is different from App resource tree. 
For App, the data is received from the backend and consists of the resources the App manages.
For AppSet, the tree consists of the Apps, that have an ownerRefrence on the AppSet.
Looks like the backend has to have a method to return the AppSet resurce tree. 

An icon is needed for AppSet (ideas might be an icon of a Factory, or the existing App icon surrounded by {} - mathematic visualization of a set of Apps, or others)
## Summary

Currently, the users have no option to view AppSets in the UI as a first-class resource (they can only view them if the AppSets themselves are managed by an App).


## Motivation

Viewing (and creating/modifying/deleting later on) from the UI would make the overall integration of AppSet as a first class ArgoCD resource complete.

### Goals

In the first phase, the goals are: support for listing, searching, filtering, viewing the details, viewing the raw AppSet manifest in details view, viewing the AppSet child resources in a tree view, and maybe deleting AppSet as well.

The full support (summary view on the details page, which translates the raw manifest into a user-friendly view, creating and updating the AppSet from the details page), can be provided on subsequent releases.

### Non-Goals

For the first phase, creating, updating and viewing user-friendly summary on details view, will not be supported.

## Proposal

Please see the attached INITIAL screenshots in ``` images ``` folder to demonstrate some of the filter fields, toolbar and sidebar options, and detailed view along with a resource tree view.

AppSet would resemble how the App looks and feels in the UI, while sticking to it's Model fields and relevant actions/filters.

This is the code with the partial implementation, based on duplication of the relevant App components (the final implementation will probably not be based on duplication but rather on extending what already exists):

https://github.com/reggie-k/argo-cd/tree/appset-ui-search

