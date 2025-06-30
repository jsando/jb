
NOTE: This project is in its infancy, it is functional only for simple projects.  Its been a low-priority side project for years, which I'm now using as a test for evaluating agentic coding (currently, Claude Code).  Please do reach out if you are interesting in contributing or would like to fund development.

# jb - Your friendly Java Build tool!

[![CI](https://github.com/jsando/jb/actions/workflows/ci.yml/badge.svg)](https://github.com/jsando/jb/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/jsando/jb)](https://goreportcard.com/report/github.com/jsando/jb)

jb ("jay-bee") wants to be the missing "batteries-included" build tool for Java developers.  It is a command line tool that takes the 
place of maven, gradle, or ant and gives you a much faster and simpler experience.

jb takes inspiration from other modern build tools such as "go", "npm", and "dotnet".  Using simple configuration files,
it provides all the typical tools a developer needs to be productive:

* Automatically download dependencies from maven repositories
* Upgrade and reporting of dependencies
* Compile code and build jar files
* Run unit tests, with optional code coverage to a nicely formatted HTML report
* Reformat code to match style guides
* Run code easily, optionally with debugging enabled
* Run tools from a development dependency
* Manage the JDK version used by each module

Things that make jb awesome:

* It's a Go static binary so it launches and runs *fast*.  Java-based build systems take much longer to start.
* It is transparent in what it does.  It shells out to the standard JDK commands like javac and jar and can show you the command being executed
* It uses simple, human-editable json files like we are used to with npm's package.json
* It relies on convention over configuration.  You don't have to use the maven folder layout if you don't want to,
  but you can if you do.
* It doesn't try to do every last thing you might need in a build.  It handles the common 80%, for the rest you 
  can use shell scripts or makefiles to build more complex layers on top of jb (as we would do with Go or npm).
* It is a little opinionated in certain things, like the format of unit test reports or the java code style.  If you can
  live with jb's choices you can be productive and move on with your work, instead of spending endless time coming 
  up with your own code style or html format.
* It uses Maven for fetching or publishing artifacts so it integrates nicely into the Java ecosystem

In short, jb aims to provide a smooth, modern development experience for Java projects.

## Installation

### Homebrew (macOS)

```bash
brew tap jsando/tools
brew install jb
```

### Download Binary

Download the latest release from the [releases page](https://github.com/jsando/jb/releases).
