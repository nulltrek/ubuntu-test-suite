#!/usr/bin/python3

import argparse

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--profile", type=str, default="none")
    args = parser.parse_args()

    print(f"profile={args.profile}")

if __name__ == "__main__":
    main()
