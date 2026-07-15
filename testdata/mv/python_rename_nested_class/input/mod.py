class Outer:
    class Nested:
        def helper(self):
            return 1

    def use(self):
        return Outer.Nested().helper()


def main():
    return Outer.Nested().helper()
