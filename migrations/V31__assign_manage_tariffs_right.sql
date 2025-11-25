UPDATE xi_users 
SET rights = array_append(rights, 'manage_tariffs'::user_right)
WHERE username = 'mairwunnx' 
  AND NOT ('manage_tariffs'::user_right = ANY(rights));