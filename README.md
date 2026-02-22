# Browser History Extractor

A simple **Go tool** to extract browser history from popular browsers like **Firefox** and **Chrome**, convert it into a CSV file, and upload it to a remote server.

## Features

- Supports extracting history from:
  - **Firefox**
  - **Chrome**
- Filter history by time range (e.g., last N days)
- Export browser history into a **CSV file**
- Upload extracted data to a remote server via HTTP

## Parameters

| Parameter    | Description                                      | Default                       |
|---------|--------------------------------------------------|-------------------------------|
| browser | The browser to target (chrome or firefox).      | firefox                        |
| time    | How many days of history to go back from today. | 30                            |
| url     | The remote HTTP(S) URL to receive the CSV file. | http://localhost:8000/upload |

## Installation

Make sure you have [Go](https://golang.org/) installed on your system. Then clone this repository and build the tool:

```bash
git clone https://github.com/drrk0/browler.git
cd browler

go build -o browler

