class Outer:
    class Inner:
        def helper(self):
            return 1

    def use(self):
        return Outer.Inner().helper()


def main():
    return Outer.Inner().helper()
