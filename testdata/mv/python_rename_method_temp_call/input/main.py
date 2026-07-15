class Box:
    def helper(self):
        return 1

    def stay(self):
        return 2


def use():
    return Box().helper() + Box().stay()


def typed(b: Box):
    return b.helper()
