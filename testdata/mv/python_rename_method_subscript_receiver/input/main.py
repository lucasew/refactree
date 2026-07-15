class Box:
    def helper(self):
        return 1

    def stay(self):
        return 2


def use(xs):
    return xs[0].helper() + xs[0].stay()
