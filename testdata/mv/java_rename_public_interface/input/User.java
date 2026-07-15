package com.example;

public class User implements ExclusionStrategy {
	public boolean shouldSkip() {
		return false;
	}
}
