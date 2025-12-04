#ifndef MARKDOWNIFY_H
#define MARKDOWNIFY_H

#ifdef __cplusplus
extern "C" {
#endif
char* markdownify(const char* markdown_text);
void free_result(char* result);
#ifdef __cplusplus
}
#endif

#endif // MARKDOWNIFY_H